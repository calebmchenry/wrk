package web

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"wrk/internal/project"
	"wrk/internal/store"
	"wrk/internal/ticket"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if m := store.Init(root); len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	write(t, root, project.ConfigPath, "version: 1\nprefix: task\ndefaults: {priority: high, labels: [default]}\nfields: {size: {type: enum, description: Shirt size, options: [S, M]}}\n")
	write(t, root, ".wrk/task-00000001.md", "---\nid: task-00000001\ntitle: Parent\nstatus: in-progress\n---\nParent body\n")
	write(t, root, ".wrk/task-00000002.md", "---\nid: task-00000002\ntitle: Child\nstatus: todo\nparent: task-00000001\ndepends_on: [task-00000001]\nlabels: [web]\nfields: {size: S, opaque: &cycle [*cycle]}\n---\n\r\n## Body 🦊\r\nNo final newline")
	write(t, root, ".wrk/task-00000003.md", "---\nid: task-00000003\ntitle: Closed\nstatus: done\n---\n")
	return root
}
func write(t *testing.T, root, path, source string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, path), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
}
func request(h http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:7331"+target, nil)
	// Simulate the origin-form requests a browser sends, not proxy requests.
	r.URL.Scheme, r.URL.Host = "", ""
	for k, v := range headers {
		if k == "Host" {
			r.Host = v
		} else {
			r.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func decode(t *testing.T, w *httptest.ResponseRecorder, status int) response {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d want %d: %s", w.Code, status, w.Body.String())
	}
	var e response
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err, w.Body.String())
	}
	if e.SchemaVersion != 1 || e.Errors == nil || e.OK != (status < 400) || (status >= 400 && e.Result != nil) {
		t.Fatalf("bad envelope: %+v", e)
	}
	return e
}
func TestReadsAndFreshValidation(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	e := decode(t, request(h, "GET", "/api/project", nil), 200)
	info := e.Result.(map[string]any)
	config := info["config"].(map[string]any)
	if *e.ProjectRoot != root || info["ticket_count"] != float64(3) || config["prefix"] != "task" || config["defaults"].(map[string]any)["priority"] != "high" || config["fields"].(map[string]any)["size"].(map[string]any)["type"] != "enum" {
		t.Fatal(e)
	}
	for path, count := range map[string]int{
		"/api/items": 2, "/api/items?all=true": 3, "/api/items?ready=true": 0,
		"/api/items?under=task-00000001&label=web": 1,
		"/api/items?label=web&label=missing":       0,
	} {
		e := decode(t, request(h, "GET", path, nil), 200)
		if len(e.Result.(map[string]any)["tickets"].([]any)) != count {
			t.Fatal(path, e)
		}
	}
	source, err := os.ReadFile(filepath.Join(root, ".wrk/task-00000002.md"))
	if err != nil {
		t.Fatal(err)
	}
	e = decode(t, request(h, "GET", "/api/items/task-00000002", nil), 200)
	item := e.Result.(map[string]any)
	summary := item["ticket"].(map[string]any)
	if item["source"] != string(source) || item["body"] != "\r\n## Body 🦊\r\nNo final newline" || summary["revision"] != ticket.Revision(source) || summary["parent"] != "task-00000001" || len(summary["blockers"].([]any)) != 1 {
		t.Fatal(e)
	}
	e = decode(t, request(h, "GET", "/api/items/task-00000001", nil), 200)
	if len(e.Result.(map[string]any)["children"].([]any)) != 1 {
		t.Fatal(e)
	}
	done := "done"
	if m := store.UpdateWithOptions(root, "task-00000001", store.UpdateOptions{Changes: ticket.Changes{Status: &done}}); len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	e = decode(t, request(h, "GET", "/api/items?ready=true", nil), 200)
	if len(e.Result.(map[string]any)["tickets"].([]any)) != 1 {
		t.Fatal(e)
	}
	// A valid requested item must never mask an unrelated invalid file.
	write(t, root, ".wrk/task-00000003.md", "invalid")
	for _, path := range []string{"/api/project", "/api/items", "/api/items/task-00000002"} {
		e = decode(t, request(h, "GET", path, nil), 503)
		if len(e.Errors) < 2 || e.Errors[0].Path == "" {
			t.Fatal(e)
		}
	}
	if err := os.Remove(filepath.Join(root, ".wrk/task-00000003.md")); err != nil {
		t.Fatal(err)
	}
	decode(t, request(h, "GET", "/api/project", nil), 200)
	write(t, root, project.ConfigPath, "version: 999")
	decode(t, request(h, "GET", "/api/project", nil), 503)
}

func TestIsolationAndRequestPolicy(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	other := fixture(t)
	for _, path := range []string{
		"/api/project?project=" + other, "/api/items?root=" + other, "/api/items?config=x",
		"/api/items/task-00000001?path=x", "/api/items?all=true&all=false",
		"/api/items?all=true&ready=true", "/api/items?all=1", "/api/items?label=",
		"/api/items?under=../foo", "/api/items?label=%ff", "/api/items?label=%XX",
	} {
		decode(t, request(h, "GET", path, nil), 400)
	}
	for _, path := range []string{
		"/api/items/task-deadbeef", "/api/items/../../etc/passwd", "/api/items/task-00000001.md",
		"/api/items/%2e%2e%2fconfig.yaml", "/api/items/task-00000001%2f", "/api/items?under=task-deadbeef",
		"/.wrk/config.yaml", "/assets/", "/etc/passwd", "//api/project", "/api/project/", "/index.html",
	} {
		decode(t, request(h, "GET", path, nil), 404)
	}
	for _, headers := range []map[string]string{
		{"Host": "evil.example:7331"}, {"Host": "localhost:7331"}, {"Host": "127.0.0.1:7332"},
		{"Origin": "https://evil.example"}, {"Origin": "null"}, {"Origin": "http://127.0.0.1:7332"},
		{"Origin": "http://127.0.0.1:7331/"}, {"Sec-Fetch-Site": "cross-site"}, {"Sec-Fetch-Site": "same-site"},
	} {
		for _, path := range []string{"/", "/api/project"} {
			w := request(h, "GET", path, headers)
			e := decode(t, w, 403)
			if e.ProjectRoot != nil || w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("cross-origin disclosure", e)
			}
		}
	}
	for _, headers := range []map[string]string{
		nil, {"Origin": "http://127.0.0.1:7331", "Sec-Fetch-Site": "same-origin"}, {"Sec-Fetch-Site": "none"},
	} {
		decode(t, request(h, "GET", "/api/project", headers), 200)
	}
	// Unsafe methods must pass the future write guard, then remain unsupported.
	for _, headers := range []map[string]string{nil, {"Origin": "http://127.0.0.1:7331"}, {"Content-Type": "application/json"}, {"Origin": "http://127.0.0.1:7331", "Content-Type": "text/plain"}} {
		decode(t, request(h, "POST", "/api/items", headers), 403)
	}
	decode(t, request(h, "POST", "/api/items", map[string]string{"Origin": "http://127.0.0.1:7331", "Content-Type": "application/json"}), 405)
	decode(t, request(h, "OPTIONS", "/api/items", nil), 403)
	for _, header := range []string{"Origin", "Sec-Fetch-Site"} {
		r := httptest.NewRequest("GET", "/api/project", nil)
		r.Host = h.authority
		r.Header[header] = []string{"http://127.0.0.1:7331", "http://evil.example"}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		decode(t, w, 403)
	}
}

func TestLimitsAndCancellation(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	decode(t, request(h, "GET", "/api/items?label="+strings.Repeat("x", 4096), nil), 414)
	for _, size := range []int64{-1, 1, MaxBodyBytes + 1} {
		r := httptest.NewRequest("GET", "/api/project", strings.NewReader("x"))
		r.Host, r.ContentLength = h.authority, size
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		status := 400
		if size > MaxBodyBytes {
			status = 413
		}
		decode(t, w, status)
	}
	for i := 0; i < MaxConcurrentReads; i++ {
		h.reads <- struct{}{}
	}
	decode(t, request(h, "GET", "/api/project", nil), 503)
	for i := 0; i < MaxConcurrentReads; i++ {
		<-h.reads
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("GET", "/api/project", nil).WithContext(ctx)
	r.Host = h.authority
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	e := decode(t, w, 503)
	if e.Errors[0].Code != "CANCELED" {
		t.Fatal(e)
	}
	write(t, root, ".wrk/task-00000004.md", strings.Repeat("x", (2<<20)+1))
	e = decode(t, request(h, "GET", "/api/project", nil), 503)
	if e.Errors[0].Code != "RESOURCE_LIMIT" {
		t.Fatal(e)
	}
}

func TestListenShutdownAndNoLock(t *testing.T) {
	root := fixture(t)
	before, err := os.ReadDir(filepath.Join(root, ".wrk"))
	if err != nil {
		t.Fatal(err)
	}
	s, ds := Listen(context.Background(), root, 0)
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	addr := s.listener.Addr().(*net.TCPAddr)
	if !addr.IP.Equal(net.IPv4(127, 0, 0, 1)) || addr.Port == 0 {
		t.Fatal(addr)
	}
	if second, ds := Listen(context.Background(), root, addr.Port); second != nil || len(ds) != 1 || ds[0].Code != "LISTEN" {
		t.Fatal(second, ds)
	}
	after, _ := os.ReadDir(filepath.Join(root, ".wrk"))
	if !reflect.DeepEqual(before, after) {
		t.Fatal("server wrote project files")
	}
	ctx, cancel := context.WithCancel(context.Background())
	ended := make(chan error, 1)
	go func() { ended <- s.Run(ctx) }()
	t.Cleanup(func() { cancel(); s.Close() })
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(s.URL + "api/project")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal(resp.Status)
	}
	title := "Changed while serving"
	if m := store.UpdateWithOptions(root, "task-00000001", store.UpdateOptions{Changes: ticket.Changes{Title: &title}}); len(m.Diagnostics) > 0 {
		t.Fatal("server holds writer lock", m.Diagnostics)
	}
	// An active handler/stream sees process cancellation and is drained.
	s2, ds := Listen(context.Background(), root, 0)
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	started := make(chan struct{})
	s2.http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() })
	ctx2, cancel2 := context.WithCancel(context.Background())
	end2 := make(chan error, 1)
	go func() { end2 <- s2.Run(ctx2) }()
	t.Cleanup(func() { cancel2(); s2.Close() })
	go func() {
		resp, err := client.Get(s2.URL)
		if err == nil {
			resp.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not start")
	}
	cancel2()
	select {
	case err := <-end2:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream shutdown timed out")
	}
	cancel()
	select {
	case err := <-ended:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timed out")
	}
	if c, err := net.DialTimeout("tcp4", addr.String(), time.Second); err == nil {
		c.Close()
		t.Fatal("listener survived")
	}
}

func TestStartupRejectsInvalidProject(t *testing.T) {
	for _, port := range []int{-1, 65536} {
		if s, ds := Listen(context.Background(), fixture(t), port); s != nil || len(ds) == 0 || ds[0].Code != "USAGE" {
			t.Fatal(s, ds)
		}
	}
	for _, bad := range []string{"ticket", "config", "missing", "symlink"} {
		t.Run(bad, func(t *testing.T) {
			root := fixture(t)
			switch bad {
			case "ticket":
				write(t, root, ".wrk/task-00000004.md", "broken")
			case "config":
				write(t, root, project.ConfigPath, "version: 42")
			case "missing":
				root = t.TempDir()
			case "symlink":
				target := filepath.Join(root, ".wrk/task-00000004.md")
				if err := os.Symlink(filepath.Join(root, project.ConfigPath), target); err != nil {
					t.Fatal(err)
				}
			}
			if s, ds := Listen(context.Background(), root, 0); s != nil || len(ds) == 0 {
				t.Fatal(s, ds)
			}
		})
	}
}
