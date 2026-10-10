package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"wrk/internal/diagnostic"
	"wrk/internal/store"
	"wrk/internal/ticket"
)

// Decode one exact object before taking the writer lock. In particular, null,
// duplicate keys, unknown keys, coercion and trailing JSON are never edits.
func mutationFields(r *http.Request, creating bool) (map[string]json.RawMessage, error) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("request must be UTF-8 JSON")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("request must be a JSON object")
	}
	fields := map[string]json.RawMessage{}
	allowed := map[string]bool{"title": true, "body": true, "labels": true, "parent": true}
	if creating {
		allowed["depends_on"] = true
		allowed["related"] = true
	} else {
		for _, key := range []string{"expected_revision", "expected_related_revision", "status", "add_dependencies", "remove_dependencies", "add_related", "remove_related", "no_related"} {
			allowed[key] = true
		}
	}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok || !allowed[name] {
			return nil, fmt.Errorf("unsupported field %q", token)
		}
		if _, exists := fields[name]; exists {
			return nil, fmt.Errorf("duplicate field %q", name)
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, err
		}
		if bytes.Equal(raw, []byte("null")) {
			return nil, fmt.Errorf("%s cannot be null; omit to preserve, use an empty string/list to clear", name)
		}
		fields[name] = raw
	}
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("request must contain exactly one object")
	}
	return fields, nil
}

func (h *handler) mutate(w http.ResponseWriter, r *http.Request, id string) {
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		fail(w, 400, &h.root, "BAD_REQUEST", "mutations do not accept query parameters")
		return
	}
	fields, err := mutationFields(r, id == "")
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			fail(w, 413, &h.root, "RESOURCE_LIMIT", "request body exceeds 1 MiB")
		} else {
			fail(w, 400, &h.root, "BAD_REQUEST", err.Error())
		}
		return
	}
	var ds []diagnostic.Diagnostic
	invalid := func(field, message string) {
		ds = append(ds, diagnostic.Diagnostic{Code: "INVALID_INPUT", Field: field, Message: message})
	}
	scalar := func(name string, max int) *string {
		raw, ok := fields[name]
		if !ok {
			return nil
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			invalid(name, "expected a string")
			return nil
		}
		if len(value) > max {
			invalid(name, fmt.Sprintf("must be at most %d UTF-8 bytes", max))
		}
		return &value
	}
	list := func(name string, ids bool) []string {
		raw, ok := fields[name]
		if !ok {
			return nil
		}
		var values []string
		if json.Unmarshal(raw, &values) != nil {
			invalid(name, "expected a list of strings")
			return nil
		}
		if len(values) > 1024 {
			invalid(name, "at most 1024 entries are supported")
			return nil
		}
		seen := map[string]bool{}
		for _, value := range values {
			if value == "" || len(value) > 4096 || (ids && !ticket.IDPattern.MatchString(value)) {
				invalid(name, "entries must be nonempty strings (at most 4096 bytes); relationships require ticket IDs")
			}
			if seen[value] {
				invalid(name, "duplicate entry: "+value)
			}
			seen[value] = true
		}
		return values
	}
	title, body := scalar("title", 4096), scalar("body", MaxBodyBytes)
	status, parent := scalar("status", 32), scalar("parent", 25)
	revision := scalar("expected_revision", 128)
	relatedRevision := scalar("expected_related_revision", 128)
	labels := list("labels", false)
	deps, add, remove := list("depends_on", true), list("add_dependencies", true), list("remove_dependencies", true)
	related, addRelated, removeRelated := list("related", true), list("add_related", true), list("remove_related", true)
	noRelated := false
	if raw, ok := fields["no_related"]; ok {
		if err := json.Unmarshal(raw, &noRelated); err != nil {
			invalid("no_related", "expected a boolean")
		}
	}
	_, addingRelated := fields["add_related"]
	_, removingRelated := fields["remove_related"]
	_, clearingRelated := fields["no_related"]
	if id != "" && (addingRelated || removingRelated || clearingRelated) && (relatedRevision == nil || *relatedRevision == "") {
		invalid("expected_related_revision", "the originally loaded related revision is required for link edits; reload and review before saving")
	}
	if id == "" && title == nil {
		invalid("title", "title is required")
	}
	if title != nil && !ticket.ValidTitle(*title) {
		invalid("title", "title must be nonblank and single-line")
	}
	if status != nil && !ticket.Statuses[*status] {
		invalid("status", "expected todo, in-progress, blocked, done, or canceled")
	}
	if parent != nil && *parent != "" && !ticket.IDPattern.MatchString(*parent) {
		invalid("parent", "expected a ticket ID, or an empty string to clear")
	}
	if id != "" && (revision == nil || *revision == "") {
		invalid("expected_revision", "the originally loaded item revision is required; reload and review before saving")
	}
	editable := false
	for name := range fields {
		if name != "expected_revision" && name != "expected_related_revision" {
			editable = true
		}
	}
	if id != "" && !editable {
		invalid("", "supply at least one editable field")
	}
	if len(ds) > 0 {
		reply(w, 400, &h.root, nil, ds)
		return
	}
	select {
	case h.writes <- struct{}{}:
		defer func() { <-h.writes }()
	default:
		w.Header().Set("Retry-After", "1")
		fail(w, 503, &h.root, "BUSY", "another browser write is active; retry after it finishes")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), ReadTimeout)
	defer cancel()
	_, labelsSet := fields["labels"]
	var m store.Mutation
	if id == "" {
		if parent != nil && *parent == "" {
			parent = nil
		}
		opts := store.CreateOptions{Title: *title, Parent: parent, Labels: labels, LabelsSet: labelsSet, Dependencies: deps, Related: related}
		if body != nil {
			opts.Body = []byte(*body)
		}
		m = store.CreateContext(ctx, h.root, opts, readLimits())
	} else {
		changes := ticket.Changes{Title: title, Body: body, Status: status, Parent: parent, Labels: labels, LabelsSet: labelsSet, AddDependencies: add, RemoveDependencies: remove, AddRelated: addRelated, RemoveRelated: removeRelated, NoRelated: noRelated}
		if parent != nil && *parent == "" {
			changes.Parent, changes.NoParent = nil, true
		}
		m = store.UpdateContext(ctx, h.root, id, store.UpdateOptions{Changes: changes, ExpectedRevision: revision, ExpectedRelatedRevision: relatedRevision}, readLimits())
	}
	h.replyMutation(w, m)
}

func (h *handler) replyMutation(w http.ResponseWriter, m store.Mutation) {
	status := http.StatusOK
	if m.Created {
		status = http.StatusCreated
	}
	if len(m.Diagnostics) > 0 {
		status = http.StatusUnprocessableEntity
		for _, d := range m.Diagnostics {
			switch d.Code {
			case "NOT_FOUND":
				status = 404
			case "CONFLICT":
				status = 409
			case "RESOURCE_LIMIT":
				status = 413
			case "BUSY", "CANCELED":
				status = 503
			case "IO", "DURABILITY_UNCERTAIN", "CLEANUP_FAILED":
				status = 500
			}
		}
	}
	// A committed error must retain the affected identity and published revision.
	// Pre-publication failures never report a candidate as saved.
	var result any
	if m.Ticket != nil && m.Snapshot != nil && (status < 400 || m.Committed) {
		publication := "unchanged"
		if m.Committed {
			publication = "committed"
		}
		values := map[string]any{"ticket": m.Snapshot.Summary(m.Ticket), "created": m.Created, "changed": m.Changed, "publication": publication}
		if m.Updates != nil {
			updates := []any{}
			for _, entry := range m.Updates {
				updates = append(updates, map[string]any{"ticket": m.Snapshot.Summary(entry.Ticket), "changed": entry.Publication == "committed", "publication": entry.Publication})
			}
			values["updates"] = updates
		}
		result = values
	}
	if m.Committed && status >= 400 {
		status = 500
	}
	if status == 503 {
		w.Header().Set("Retry-After", "1")
	}
	reply(w, status, &h.root, result, m.Diagnostics)
}

func mutationRoute(path string) (id string, allowed string) {
	if path == "/api/items" {
		return "", "GET, HEAD, POST"
	}
	id = strings.TrimPrefix(path, "/api/items/")
	if strings.HasPrefix(path, "/api/items/") && ticket.IDPattern.MatchString(id) {
		return id, "GET, HEAD, PATCH"
	}
	return "", "GET, HEAD"
}
