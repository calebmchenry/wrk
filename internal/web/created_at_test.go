package web

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"wrk/internal/project"
)

func TestHTTPCreatedAt(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	start := time.Now().UTC().Truncate(time.Second)
	created := decode(t, writeRequest(h, "POST", "/api/items", `{"title":"Timestamped","body":"description","parent":"task-00000001","depends_on":["task-00000003"],"related":["task-00000002"],"labels":["tag"]}`), 201)
	summary := created.Result.(map[string]any)["ticket"].(map[string]any)
	id := summary["id"].(string)
	date, ok := summary["created_at"].(string)
	parsed, err := time.Parse(time.RFC3339, date)
	if !ok || err != nil || !strings.HasSuffix(date, "Z") || parsed.Before(start) || parsed.After(time.Now()) {
		t.Fatalf("creation date: %+v (%v)", summary, err)
	}
	child := decode(t, writeRequest(h, "POST", "/api/items", `{"title":"Nested","parent":"`+id+`","depends_on":["`+id+`"],"related":["`+id+`"]}`), 201)
	childSummary := child.Result.(map[string]any)["ticket"].(map[string]any)
	childID := childSummary["id"].(string)
	want := map[string]any{id: date, childID: childSummary["created_at"]}
	if want[childID] == nil {
		t.Fatal("missing child's creation timestamp")
	}
	var checkSummaries func(any) int
	checkSummaries = func(value any) int {
		t.Helper()
		count := 0
		switch value := value.(type) {
		case map[string]any:
			if _, summary := value["revision"]; summary {
				id := value["id"].(string)
				got, exists := value["created_at"]
				if !exists || got != want[id] {
					t.Fatalf("%s: created_at=%v, exists=%t, want %v", id, got, exists, want[id])
				}
				count++
			}
			for _, child := range value {
				count += checkSummaries(child)
			}
		case []any:
			for _, child := range value {
				count += checkSummaries(child)
			}
		}
		return count
	}
	for _, path := range []string{"/api/items?all=true", "/api/items/" + id, "/api/items/task-00000001", "/api/workspace?selected=" + id} {
		response := decode(t, request(h, "GET", path, nil), 200)
		if count := checkSummaries(response.Result); count < 2 {
			t.Fatalf("%s: checked only %d summaries", path, count)
		}
	}
	// Reject attempts to set, replace or clear the server-owned field, even
	// when the submitted value matches the saved timestamp.
	before := project.Load(root)
	for _, value := range []any{date, "2000-01-01T00:00:00Z", nil, 123} {
		body, _ := json.Marshal(map[string]any{"title": "Rejected", "created_at": value})
		for _, response := range []response{
			decode(t, writeRequest(h, "POST", "/api/items", string(body)), 400),
			decode(t, patchRequest(t, h, id, map[string]any{"expected_revision": revision(t, root, id), "created_at": value, "status": "done"}), 400),
		} {
			if !strings.Contains(response.Errors[0].Message, "created_at") {
				t.Fatal(response.Errors)
			}
		}
	}
	if err := before.Compare(); err != nil {
		t.Fatal("rejected timestamp writes changed files:", err)
	}
	for _, target := range []string{id, "task-00000002"} {
		for _, changes := range []map[string]any{
			{"body": "edited", "title": "Edited", "labels": []string{"new"}},
			{"status": "done"}, {"parent": "task-00000003", "add_dependencies": []string{"task-00000003"}},
			{"parent": "", "remove_dependencies": []string{"task-00000003"}},
			{"no_related": true},
		} {
			changes["expected_revision"] = revision(t, root, target)
			changes["expected_related_revision"] = project.Load(root).RelatedRevision(target)
			response := decode(t, patchRequest(t, h, target, changes), 200)
			if checkSummaries(response.Result) == 0 {
				t.Fatal("missing mutation summary")
			}
			current := project.Load(root)
			if len(current.Diagnostics) > 0 || current.ByID[id].CreatedAt == nil || *current.ByID[id].CreatedAt != date || current.ByID["task-00000002"].CreatedAt != nil {
				t.Fatal("edits changed creation dates", current.Diagnostics)
			}
		}
	}
}
