package web

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"wrk/internal/project"
	"wrk/internal/store"
	"wrk/internal/ticket"
)

func TestWorkspaceDetectionAndRecovery(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	path := "/api/workspace?selected=task-00000002"
	first := request(h, "GET", path, nil)
	decode(t, first, 200)
	tag := first.Header().Get("ETag")
	if tag == "" {
		t.Fatal("missing workspace validator")
	}
	unchanged := func() {
		t.Helper()
		w := request(h, "GET", path, map[string]string{"If-None-Match": tag})
		if w.Code != 304 || w.Body.Len() != 0 || w.Header().Get("ETag") != tag {
			t.Fatal(w.Code, w.Body.String(), w.Header())
		}
	}
	changed := func() map[string]any {
		t.Helper()
		w := request(h, "GET", path, map[string]string{"If-None-Match": tag})
		data := decode(t, w, 200).Result.(map[string]any)
		if next := w.Header().Get("ETag"); next == tag || next == "" {
			t.Fatal("failed to detect changed input", next)
		} else {
			tag = next
		}
		return data
	}
	unchanged()
	// Readers neither wait for a writer nor mutate files while it owns the lock.
	lock, err := store.Acquire(filepath.Join(root, ".wrk"))
	if err != nil {
		t.Fatal(err)
	}
	before := project.Load(root)
	unchanged()
	if err := before.Compare(); err != nil {
		t.Fatal("read changed files", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".wrk-stage-task-00000002.md", "AGENTS.md", "runtime.log", ".lock"} {
		write(t, root, ".wrk/"+name, "ignored churn")
	}
	unchanged()
	// Metadata from the shared CLI store is published by atomic replacement.
	status, parent, body := "done", "task-00000003", "## New description\n"
	m := store.UpdateWithOptions(root, "task-00000002", store.UpdateOptions{Changes: ticket.Changes{
		Status: &status, Parent: &parent, AddLabels: []string{"live"}, Body: &body,
		RemoveDependencies: []string{"task-00000001"}, AddDependencies: []string{"task-00000003"},
	}})
	if len(m.Diagnostics) != 0 {
		t.Fatal(m.Diagnostics)
	}
	data := changed()
	detail := data["detail"].(map[string]any)
	summary := detail["ticket"].(map[string]any)
	if summary["status"] != status || summary["parent"] != parent || detail["body"] != body || len(summary["blockers"].([]any)) != 0 || !reflect.DeepEqual(summary["labels"], []any{"live", "web"}) {
		t.Fatal(data)
	}
	if detail["dependencies"].([]any)[0].(map[string]any)["id"] != parent {
		t.Fatal(detail)
	}
	// Same-size direct edit with restored mtime still changes the byte validator.
	file := filepath.Join(root, ".wrk/task-00000002.md")
	original, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, ".wrk/task-00000002.md", strings.Replace(string(original), "New description", "New information", 1))
	if err := os.Chtimes(file, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(changed()["detail"].(map[string]any)["body_html"].(string), "New information") {
		t.Fatal("direct body edit not rendered")
	}
	write(t, root, project.ConfigPath, "version: 1\nprefix: updated\nfields: {size: {type: string, description: Updated field}}\n")
	if changed()["project"].(map[string]any)["config"].(map[string]any)["prefix"] != "updated" {
		t.Fatal("config not refreshed")
	}
	write(t, root, ".wrk/task-00000004.md", "---\nid: task-00000004\ntitle: Created\nstatus: todo\n---\n")
	if len(changed()["tickets"].([]any)) != 4 {
		t.Fatal("creation not detected")
	}
	// Invalid intermediate states must fail even with the last healthy validator.
	for _, input := range []struct{ path, invalid string }{
		{".wrk/task-00000004.md", "broken"},
		{project.ConfigPath, "version: ["},
	} {
		valid, err := os.ReadFile(filepath.Join(root, input.path))
		if err != nil {
			t.Fatal(err)
		}
		write(t, root, input.path, input.invalid)
		w := request(h, "GET", path, map[string]string{"If-None-Match": tag})
		decode(t, w, 503)
		if w.Header().Get("ETag") != "" {
			t.Fatal("invalid state advertised healthy validator")
		}
		write(t, root, input.path, string(valid))
		unchanged()
		decode(t, request(h, "GET", path, nil), 200) // Reconnect's unconditional full snapshot.
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	data = changed()
	if data["detail"] != nil || len(data["tickets"].([]any)) != 3 || data["selected"] != "task-00000002" {
		t.Fatal("selected deletion must retain selection and refresh the list", data)
	}
}

func TestWorkspaceSelectionAndValidatorIsolation(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	first := request(h, "GET", "/api/workspace", nil)
	tag := first.Header().Get("ETag")
	for _, id := range []string{"task-00000001", "task-00000002", "task-deadbeef"} {
		path := "/api/workspace?selected=" + id
		w := request(h, "GET", path, map[string]string{"If-None-Match": tag})
		data := decode(t, w, 200).Result.(map[string]any)
		if data["selected"] != id || w.Header().Get("ETag") == tag {
			t.Fatal(data, w.Header())
		}
		if id != "task-deadbeef" {
			want := decode(t, request(h, "GET", "/api/items/"+id, nil), 200).Result
			if !reflect.DeepEqual(data["detail"], want) {
				t.Fatal("workspace and item detail differ", data)
			}
		}
	}
	for _, query := range []string{"selected=", "selected=../file", "selected=task-00000001&selected=task-00000002", "selected=%XX", "root=x", "all=true"} {
		decode(t, request(h, "GET", "/api/workspace?"+query, nil), 400)
	}
	decode(t, request(h, "GET", "/api/workspace?selected=task-00000001", map[string]string{"Origin": "https://evil.example", "If-None-Match": tag}), 403)
	if w := request(h, "HEAD", "/api/workspace", map[string]string{"If-None-Match": tag}); w.Code != 304 || w.Body.Len() != 0 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(h, "GET", "/live.mjs", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "createPoller") {
		t.Fatal("live module not embedded", w.Code)
	}
}
