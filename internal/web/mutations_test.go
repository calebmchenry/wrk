package web

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wrk/internal/diagnostic"
	"wrk/internal/project"
	"wrk/internal/store"
	"wrk/internal/ticket"
)

func writeRequest(h *handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = h.authority
	r.Header.Set("Origin", "http://"+h.authority)
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func revision(t *testing.T, root, id string) string {
	t.Helper()
	s := project.Load(root)
	if len(s.Diagnostics) > 0 {
		t.Fatal(s.Diagnostics)
	}
	return ticket.Revision(s.ByID[id].Source)
}
func patchRequest(t *testing.T, h *handler, id string, changes map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(changes)
	if err != nil {
		t.Fatal(err)
	}
	return writeRequest(h, "PATCH", "/api/items/"+id, string(data))
}
func TestHTTPCreateAndEdit(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	created := decode(t, writeRequest(h, "POST", "/api/items", `{"title":"From browser"}`), 201)
	result := created.Result.(map[string]any)
	summary := result["ticket"].(map[string]any)
	id := summary["id"].(string)
	if result["publication"] != "committed" || summary["status"] != "todo" || summary["priority"] != "high" || summary["labels"].([]any)[0] != "default" {
		t.Fatal(created)
	}
	created = decode(t, writeRequest(h, "POST", "/api/items", `{"title":"Complete create","body":"\r\n# Hello 🦊\nNo final newline","labels":[],"parent":"task-00000001","depends_on":["task-00000003"]}`), 201)
	s := project.Load(root)
	c := s.ByID[created.Result.(map[string]any)["ticket"].(map[string]any)["id"].(string)]
	if string(c.Body) != "\r\n# Hello 🦊\nNo final newline" || len(c.Labels) != 0 || *c.Parent != "task-00000001" || c.DependsOn[0] != "task-00000003" {
		t.Fatal(c)
	}
	// Shared patching preserves recursive custom YAML, priority, permissions, and
	// exact omitted body bytes. Every status is explicit, including reopening.
	for _, status := range []string{"in-progress", "blocked", "done", "canceled", "todo"} {
		decode(t, patchRequest(t, h, "task-00000002", map[string]any{"expected_revision": revision(t, root, "task-00000002"), "status": status}), 200)
	}
	before := project.Load(root).ByID["task-00000002"]
	decode(t, patchRequest(t, h, "task-00000002", map[string]any{"expected_revision": ticket.Revision(before.Source), "title": "Edited", "labels": []string{"new", "label with spaces"}, "parent": "", "add_dependencies": []string{id}, "remove_dependencies": []string{"task-00000001"}}), 200)
	after := project.Load(root).ByID["task-00000002"]
	if after.Title != "Edited" || string(before.Body) != string(after.Body) || after.Parent != nil || after.DependsOn[0] != id || after.Priority != before.Priority || !strings.Contains(string(after.Source), "opaque:") || !strings.Contains(string(after.Source), "*") {
		t.Fatal(string(after.Source))
	}
	info, _ := os.Stat(filepath.Join(root, after.Path))
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	decode(t, patchRequest(t, h, after.ID, map[string]any{"expected_revision": ticket.Revision(after.Source), "body": "", "labels": []string{}}), 200)
	after = project.Load(root).ByID[after.ID]
	if len(after.Body) != 0 || len(after.Labels) != 0 {
		t.Fatal(after)
	}
	noop := decode(t, patchRequest(t, h, after.ID, map[string]any{"expected_revision": ticket.Revision(after.Source), "title": after.Title}), 200)
	if noop.Result.(map[string]any)["publication"] != "unchanged" || revision(t, root, after.ID) != ticket.Revision(after.Source) {
		t.Fatal(noop)
	}
}

func TestHTTPStaleWritesAndGraphErrors(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	id := "task-00000001"
	base := revision(t, root, id)
	title := "Agent edited"
	if m := store.Update(root, id, &title, nil); len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	current := project.Load(root)
	// A stale no-op is rejected, too; no acknowledgement of an obsolete base.
	e := decode(t, patchRequest(t, h, id, map[string]any{"expected_revision": base, "title": title}), 409)
	if e.Errors[0].Code != "CONFLICT" {
		t.Fatal(e)
	}
	if err := current.Compare(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []map[string]any{
		{"parent": "task-00000002"}, {"add_dependencies": []string{"task-00000002"}},
		{"parent": "task-deadbeef"}, {"add_dependencies": []string{"task-deadbeef"}},
		{"parent": id}, {"add_dependencies": []string{id}},
	} {
		change["expected_revision"] = revision(t, root, id)
		e := decode(t, patchRequest(t, h, id, change), 422)
		if e.Errors[0].Field == "" {
			t.Fatal("missing actionable field", e)
		}
		if err := current.Compare(); err != nil {
			t.Fatal(err)
		}
	}
	// Unrelated edits do not invalidate an item token.
	store.Update(root, "task-00000003", &title, nil)
	decode(t, patchRequest(t, h, id, map[string]any{"expected_revision": revision(t, root, id), "title": "Browser after review"}), 200)
	base = revision(t, root, id)
	write(t, root, ".wrk/"+id+".md", "malformed external rewrite")
	decode(t, patchRequest(t, h, id, map[string]any{"expected_revision": base, "status": "done"}), 409)
	if err := os.Remove(filepath.Join(root, ".wrk/"+id+".md")); err != nil {
		t.Fatal(err)
	}
	decode(t, patchRequest(t, h, id, map[string]any{"expected_revision": base, "status": "done"}), 404)
}

func TestHTTPWriteInputPolicy(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	before := project.Load(root)
	for _, body := range []string{
		``, `null`, `[]`, `{}`, `{"title":"a","title":"b"}`, `{"Title":"x"}`, `{"title":"x","root":"/tmp"}`,
		`{"title":"x"} {}`, `{"title":3}`, `{"title":null}`, `{"title":"  "}`, `{"title":"a\nb"}`,
		`{"title":"x","labels":null}`, `{"title":"x","labels":[null]}`, `{"title":"x","labels":["a","a"]}`,
		`{"title":"x","depends_on":["../path"]}`, `{"title":"x","parent":false}`, `{"title":"x","body":{}}`,
		`{"title":"x","priority":"urgent"}`, `{"title":"x","status":"done"}`, `{"title":"` + string([]byte{0xff}) + `"}`,
		`{"title":"` + strings.Repeat("x", 4097) + `"}`,
	} {
		decode(t, writeRequest(h, "POST", "/api/items", body), 400)
	}
	for _, body := range []string{`{"title":"x"}`, `{"expected_revision":null,"title":"x"}`, `{"expected_revision":""}`, `{"expected_revision":"x"}`, `{"expected_revision":"x","status":"bad"}`, `{"expected_revision":"x","depends_on":[]}`} {
		decode(t, writeRequest(h, "PATCH", "/api/items/task-00000001", body), 400)
	}
	decode(t, writeRequest(h, "POST", "/api/items?project=other", `{"title":"x"}`), 400)
	decode(t, writeRequest(h, "PATCH", "/api/items/%74ask-00000001", `{}`), 404)
	decode(t, writeRequest(h, "PUT", "/api/items/task-00000001", `{}`), 405)
	decode(t, writeRequest(h, "POST", "/api/workspace", `{}`), 405)
	// Known and chunked body limits are both enforced before the store runs.
	for _, chunked := range []bool{false, true} {
		r := httptest.NewRequest("POST", "/api/items", strings.NewReader(`{"title":"`+strings.Repeat("x", MaxBodyBytes)+`"}`))
		r.Host = h.authority
		r.Header.Set("Origin", "http://"+h.authority)
		r.Header.Set("Content-Type", "application/json")
		if chunked {
			r.ContentLength = -1
			r.TransferEncoding = []string{"chunked"}
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		decode(t, w, 413)
	}
	if err := before.Compare(); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPWriteSecurityAndBusy(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	before := project.Load(root)
	for _, bad := range []struct{ key, value string }{
		{"Origin", ""}, {"Origin", "null"}, {"Origin", "http://evil.example"}, {"Origin", "http://127.0.0.1:7332"},
		{"Content-Type", "text/plain"}, {"Content-Type", "application/x-www-form-urlencoded"}, {"Content-Type", "multipart/form-data; boundary=x"}, {"Content-Type", "application/json; charset=latin1"},
		{"Host", "evil.example:7331"}, {"Sec-Fetch-Site", "same-site"}, {"Sec-Fetch-Site", "cross-site"},
	} {
		r := httptest.NewRequest("POST", "/api/items", strings.NewReader(`{"title":"Unwanted"}`))
		r.Host = h.authority
		r.Header.Set("Origin", "http://"+h.authority)
		r.Header.Set("Content-Type", "application/json")
		if bad.key == "Host" {
			r.Host = bad.value
		} else {
			r.Header.Set(bad.key, bad.value)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		e := decode(t, w, 403)
		if e.ProjectRoot != nil || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal(e)
		}
	}
	decode(t, request(h, "GET", "/api/items?title=Unwanted", nil), 400)
	lock, err := store.Acquire(filepath.Join(root, ".wrk"))
	if err != nil {
		t.Fatal(err)
	}
	e := decode(t, writeRequest(h, "POST", "/api/items", `{"title":"Busy"}`), 503)
	if e.Errors[0].Code != "BUSY" {
		t.Fatal(e)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	h.writes <- struct{}{}
	decode(t, writeRequest(h, "POST", "/api/items", `{"title":"Busy"}`), 503)
	<-h.writes
	if err := before.Compare(); err != nil {
		t.Fatal(err)
	}
	// The locked load, not a preliminary unbounded store load, enforces limits.
	write(t, root, ".wrk/task-00000004.md", strings.Repeat("x", (2<<20)+1))
	e = decode(t, writeRequest(h, "POST", "/api/items", `{"title":"Too large"}`), 413)
	if e.Errors[0].Code != "RESOURCE_LIMIT" {
		t.Fatal(e)
	}
}

func TestHTTPCommittedErrorEnvelope(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	m := store.Create(root, store.CreateOptions{Title: "Published"})
	m.Diagnostics = []diagnostic.Diagnostic{diagnostic.New("DURABILITY_UNCERTAIN", "directory sync failed after publication", m.Ticket.Path)}
	w := httptest.NewRecorder()
	h.replyMutation(w, m)
	var e response
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if w.Code != 500 || e.OK || e.Result.(map[string]any)["publication"] != "committed" || e.Result.(map[string]any)["ticket"].(map[string]any)["revision"] != ticket.Revision(m.Ticket.Source) {
		t.Fatal(w.Code, e)
	}
	// Cancellation before the locked load never writes.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("POST", "/api/items", strings.NewReader(`{"title":"Canceled"}`)).WithContext(ctx)
	r.Host = h.authority
	r.Header.Set("Origin", "http://"+h.authority)
	r.Header.Set("Content-Type", "application/json")
	before := project.Load(root)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	decode(t, w, 503)
	if err := before.Compare(); err != nil {
		t.Fatal(err)
	}
}
