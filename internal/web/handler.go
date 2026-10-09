package web

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"wrk/internal/diagnostic"
	"wrk/internal/project"
	"wrk/internal/ticket"
)

//go:embed assets/index.html assets/app.js assets/model.mjs assets/live.mjs assets/style.css
var assets embed.FS

type response struct {
	SchemaVersion int                     `json:"schema_version"`
	OK            bool                    `json:"ok"`
	ProjectRoot   *string                 `json:"project_root"`
	Result        any                     `json:"result"`
	Errors        []diagnostic.Diagnostic `json:"errors"`
}

type handler struct {
	root, authority string
	reads           chan struct{}
}

func newHandler(root, authority string) *handler {
	return &handler{root: root, authority: authority, reads: make(chan struct{}, MaxConcurrentReads)}
}

func reply(w http.ResponseWriter, status int, root *string, result any, ds []diagnostic.Diagnostic) {
	if ds == nil {
		ds = []diagnostic.Diagnostic{}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response{1, status < 400, root, result, ds})
}

func fail(w http.ResponseWriter, status int, root *string, code, message string) {
	reply(w, status, root, nil, []diagnostic.Diagnostic{diagnostic.New(code, message, "")})
}

// checkOrigin is shared by all routes, including future mutation endpoints.
// Unsafe methods require an explicit exact Origin and JSON content type; no
// CORS exceptions, same-site exceptions, or Referer fallback are allowed.
func (h *handler) checkOrigin(r *http.Request) bool {
	if r.Host != h.authority || r.URL.IsAbs() {
		return false
	}
	origins := r.Header.Values("Origin")
	if len(origins) > 1 || (len(origins) == 1 && origins[0] != "http://"+h.authority) {
		return false
	}
	sites := r.Header.Values("Sec-Fetch-Site")
	if len(sites) > 1 {
		return false
	}
	if len(sites) == 1 && sites[0] != "same-origin" && sites[0] != "none" {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if len(origins) != 1 {
			return false
		}
		typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || typ != "application/json" {
			return false
		}
	}
	return true
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	if !h.checkOrigin(r) {
		fail(w, http.StatusForbidden, nil, "FORBIDDEN", "use the printed local URL and same-origin requests")
		return
	}
	if len(r.URL.RequestURI()) > 4096 {
		fail(w, http.StatusRequestURITooLong, &h.root, "RESOURCE_LIMIT", "request URL exceeds 4096 bytes")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	if r.ContentLength > MaxBodyBytes {
		w.Header().Set("Connection", "close")
		fail(w, http.StatusRequestEntityTooLarge, &h.root, "RESOURCE_LIMIT", "request body exceeds 1 MiB")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		fail(w, http.StatusMethodNotAllowed, &h.root, "METHOD_NOT_ALLOWED", "only GET and HEAD reads are available")
		return
	}
	if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
		w.Header().Set("Connection", "close")
		fail(w, http.StatusBadRequest, &h.root, "BAD_REQUEST", "read requests must not have a body")
		return
	}
	// No redirect/clean-path router or filesystem server: only these exact assets.
	if r.URL.RawPath != "" {
		fail(w, http.StatusNotFound, &h.root, "NOT_FOUND", "unknown route or invalid item ID")
		return
	}
	if file, ok := map[string]string{"/": "index.html", "/app.js": "app.js", "/model.mjs": "model.mjs", "/live.mjs": "live.mjs", "/style.css": "style.css"}[r.URL.Path]; ok {
		if r.URL.RawQuery != "" {
			fail(w, 400, &h.root, "BAD_REQUEST", "this route does not accept query parameters")
			return
		}
		data, _ := assets.ReadFile("assets/" + file)
		contentType := map[string]string{"index.html": "text/html; charset=utf-8", "app.js": "text/javascript; charset=utf-8", "model.mjs": "text/javascript; charset=utf-8", "live.mjs": "text/javascript; charset=utf-8", "style.css": "text/css; charset=utf-8"}[file]
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
		return
	}
	path := r.URL.Path
	id := strings.TrimPrefix(path, "/api/items/")
	isItem := strings.HasPrefix(path, "/api/items/") && ticket.IDPattern.MatchString(id)
	if path != "/api/project" && path != "/api/workspace" && path != "/api/items" && !isItem {
		fail(w, 404, &h.root, "NOT_FOUND", "unknown route or invalid item ID")
		return
	}
	var filters listFilters
	var selected string
	var err error
	if path == "/api/workspace" {
		selected, err = parseSelection(r.URL)
	} else {
		filters, err = parseFilters(r.URL, path == "/api/items")
	}
	if err != nil {
		fail(w, 400, &h.root, "BAD_REQUEST", err.Error())
		return
	}
	select {
	case h.reads <- struct{}{}:
		defer func() { <-h.reads }()
	default:
		w.Header().Set("Retry-After", "1")
		fail(w, 503, &h.root, "BUSY", "too many concurrent reads; retry shortly")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), ReadTimeout)
	defer cancel()
	s := project.LoadContext(ctx, h.root, readLimits())
	if len(s.Diagnostics) > 0 {
		reply(w, 503, &h.root, nil, s.Diagnostics)
		return
	}
	if ctx.Err() != nil {
		fail(w, 503, &h.root, "CANCELED", "read canceled; retry shortly")
		return
	}
	switch {
	case path == "/api/project":
		reply(w, 200, &h.root, projectInfo(s), nil)
	case path == "/api/workspace":
		// Always load and validate before checking the validator. Only ticket and
		// config bytes contribute, never timestamps, locks, or staging files.
		etag := workspaceTag(s, selected)
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		var detail any
		if t := s.ByID[selected]; t != nil {
			detail, err = itemDetail(s, t)
			if err != nil {
				fail(w, 500, &h.root, "RENDER", "unable to render item description")
				return
			}
		}
		items := []workspaceItem{}
		for _, summary := range s.List(true, false) {
			items = append(items, workspaceItem{Summary: summary, Body: string(s.ByID[summary.ID].Body)})
		}
		reply(w, 200, &h.root, map[string]any{"project": projectInfo(s), "tickets": items, "selected": selected, "detail": detail}, nil)
	case path == "/api/items":
		if filters.under != nil && s.ByID[*filters.under] == nil {
			fail(w, 404, &h.root, "NOT_FOUND", "scope root not found in this project")
			return
		}
		reply(w, 200, &h.root, map[string]any{"tickets": s.ScopedList(filters.all, filters.ready, filters.labels, filters.under)}, nil)
	case isItem:
		t := s.ByID[id]
		if t == nil {
			fail(w, 404, &h.root, "NOT_FOUND", "ticket not found in this project")
			return
		}
		detail, err := itemDetail(s, t)
		if err != nil {
			fail(w, 500, &h.root, "RENDER", "unable to render item description")
			return
		}
		reply(w, 200, &h.root, detail, nil)
	}
}

// A missing selected ticket is a successful snapshot with a null detail, so
// deletion does not prevent the rest of the workspace from refreshing.
func parseSelection(u *url.URL) (string, error) {
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", fmt.Errorf("invalid query: %w", err)
	}
	for name, values := range q {
		if name != "selected" || len(values) != 1 || !ticket.IDPattern.MatchString(values[0]) {
			return "", fmt.Errorf("workspace accepts only one selected ticket ID")
		}
	}
	return q.Get("selected"), nil
}

func workspaceTag(s *project.Snapshot, selected string) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "%d:%s", len(selected), selected)
	paths := make([]string, 0, len(s.Files))
	for path := range s.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		data := s.Files[path].Data
		fmt.Fprintf(hash, "%d:%s%d:", len(path), path, len(data))
		_, _ = hash.Write(data)
	}
	return fmt.Sprintf(`"%x"`, hash.Sum(nil))
}

func itemDetail(s *project.Snapshot, t *ticket.Ticket) (any, error) {
	bodyHTML, err := renderMarkdown(t.Body)
	if err != nil {
		return nil, err
	}
	dependencies, dependents := []project.Summary{}, []project.Summary{}
	var parent *project.Summary
	if t.Parent != nil {
		p := s.Summary(s.ByID[*t.Parent])
		parent = &p
	}
	for _, dep := range s.Summary(t).DependsOn {
		dependencies = append(dependencies, s.Summary(s.ByID[dep]))
	}
	for _, other := range s.List(true, false) {
		for _, dep := range other.DependsOn {
			if dep == t.ID {
				dependents = append(dependents, other)
			}
		}
	}
	return map[string]any{
		"ticket": s.Summary(t), "body": string(t.Body), "body_html": bodyHTML,
		"source": string(t.Source), "metadata": string(t.Source[:len(t.Source)-len(t.Body)]),
		"parent": parent, "children": s.Children(t.ID), "dependencies": dependencies, "dependents": dependents,
	}, nil
}

type workspaceItem struct {
	project.Summary
	Body string `json:"body"`
}

type listFilters struct {
	all, ready bool
	labels     []string
	under      *string
}

func parseFilters(u *url.URL, list bool) (listFilters, error) {
	f := listFilters{}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return f, fmt.Errorf("invalid query: %w", err)
	}
	for name, values := range q {
		if !list || (name != "all" && name != "ready" && name != "label" && name != "under") {
			return f, fmt.Errorf("unsupported query parameter %q; the project is fixed at startup", name)
		}
		if name != "label" && len(values) != 1 {
			return f, fmt.Errorf("%s must occur once", name)
		}
		switch name {
		case "all", "ready":
			if values[0] != "true" && values[0] != "false" {
				return f, fmt.Errorf("%s must be true or false", name)
			}
			if name == "all" {
				f.all = values[0] == "true"
			} else {
				f.ready = values[0] == "true"
			}
		case "under":
			if !ticket.IDPattern.MatchString(values[0]) {
				return f, fmt.Errorf("under must be a ticket ID")
			}
			f.under = &values[0]
		case "label":
			for _, v := range values {
				if v == "" || !utf8.ValidString(v) {
					return f, fmt.Errorf("label must be a nonempty UTF-8 string")
				}
			}
			f.labels = values
		}
	}
	if f.all && f.ready {
		return f, fmt.Errorf("all and ready conflict")
	}
	return f, nil
}

func projectInfo(s *project.Snapshot) any {
	fields := map[string]any{}
	for name, def := range s.Config.Fields {
		fields[name] = map[string]any{"type": def.Type, "description": def.Description, "options": append([]string{}, def.Options...)}
	}
	return map[string]any{
		"ticket_count": len(s.Tickets),
		"config": map[string]any{
			"version": 1, "prefix": s.Config.Prefix,
			"defaults": map[string]any{"priority": s.Config.Priority, "labels": s.Config.Labels}, "fields": fields,
		},
		"statuses":   []string{"todo", "in-progress", "blocked", "done", "canceled"},
		"priorities": []string{"low", "normal", "high", "urgent"},
	}
}
