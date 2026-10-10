package web

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"wrk/internal/diagnostic"
	"wrk/internal/project"
	"wrk/internal/store"
	"wrk/internal/ticket"
)

func TestHTTPRelatedParityAndStaleIncoming(t *testing.T) {
	root := fixture(t)
	h := newHandler(root, "127.0.0.1:7331")
	id, other := "task-00000001", "task-00000003"
	patch := func(target string, changes map[string]any, code int) response {
		t.Helper()
		s := project.Load(root)
		changes["expected_revision"] = s.Summary(s.ByID[target]).Revision
		changes["expected_related_revision"] = s.RelatedRevision(target)
		return decode(t, patchRequest(t, h, target, changes), code)
	}
	e := decode(t, writeRequest(h, "POST", "/api/items", `{"title":"Linked","related":["task-00000001"]}`), 201)
	created := e.Result.(map[string]any)["ticket"].(map[string]any)["id"].(string)
	patch(other, map[string]any{"add_related": []string{id, created}}, 200)
	// Closing a contextual cycle does not invoke the dependency cycle checker.
	for _, target := range []string{id, created} {
		e = patch(target, map[string]any{"add_related": []string{other}}, 200)
		if e.Result.(map[string]any)["publication"] != "unchanged" {
			t.Fatal("reverse add not a no-op", e)
		}
	}
	s := project.Load(root)
	summary := s.Summary(s.ByID[id])
	detail, err := itemDetail(s, s.ByID[id])
	if err != nil {
		t.Fatal(err)
	}
	related := detail.(map[string]any)["related"].([]project.Summary)
	if len(related) != 2 || !reflect.DeepEqual(summary.Related, s.RelatedIDs(id)) {
		t.Fatal(detail)
	}
	patch(other, map[string]any{"remove_related": []string{id}}, 200)
	// Incoming removal leaves id's exact bytes unchanged, yet stale saves fail.
	e = decode(t, patchRequest(t, h, id, map[string]any{"expected_revision": summary.Revision, "expected_related_revision": summary.RelatedRevision, "no_related": true}), 409)
	if e.Errors[0].Code != "CONFLICT" || revision(t, root, id) != summary.Revision {
		t.Fatal(e)
	}
	for _, invalid := range []map[string]any{
		{"add_related": []string{id}}, {"remove_related": []string{id}},
		{"add_related": []string{"task-deadbeef"}}, {"remove_related": []string{"task-deadbeef"}},
		{"add_related": []string{other}, "remove_related": []string{other}},
		{"no_related": true, "add_related": []string{other}},
	} {
		patch(id, invalid, 422)
	}
	for _, invalid := range []map[string]any{
		{"add_related": []string{other}}, {"add_related": []string{other, other}, "expected_related_revision": summary.RelatedRevision},
		{"no_related": "yes", "expected_related_revision": summary.RelatedRevision},
		{"related": []string{}}, {"remove_related": nil},
	} {
		invalid["expected_revision"] = revision(t, root, id)
		decode(t, patchRequest(t, h, id, invalid), 400)
	}
	// Removing an incoming edge edits its owner, while ordinary changes affect id.
	e = patch(id, map[string]any{"no_related": true, "title": "Combined"}, 200)
	if len(e.Result.(map[string]any)["updates"].([]any)) != 2 {
		t.Fatal(e)
	}
	s = project.Load(root)
	if len(s.RelatedIDs(id)) != 0 || s.ByID[id].Title != "Combined" || len(s.RelatedIDs(created)) != 1 {
		t.Fatal("clear affected unrelated link")
	}
	e = patch(id, map[string]any{"no_related": true}, 200)
	if e.Result.(map[string]any)["publication"] != "unchanged" {
		t.Fatal(e)
	}
}

func TestHTTPRelatedPartialPublicationEnvelope(t *testing.T) {
	root := fixture(t)
	s := project.Load(root)
	a, b := s.ByID["task-00000001"], s.ByID["task-00000002"]
	h := newHandler(root, "127.0.0.1:7331")
	w := httptest.NewRecorder()
	h.replyMutation(w, store.Mutation{Ticket: a, Snapshot: s, Changed: true, Committed: true,
		Updates:     []store.UpdateEntry{{Ticket: b, Publication: "committed"}, {Ticket: a, Publication: "pending"}},
		Diagnostics: []diagnostic.Diagnostic{diagnostic.New("IO", "injected rename failure", a.Path)}})
	var e response
	if w.Code != 500 || json.Unmarshal(w.Body.Bytes(), &e) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	result := e.Result.(map[string]any)
	entries := result["updates"].([]any)
	if result["publication"] != "committed" || entries[0].(map[string]any)["publication"] != "committed" || entries[1].(map[string]any)["publication"] != "pending" {
		t.Fatal(e)
	}
	if result["ticket"].(map[string]any)["revision"] != ticket.Revision(a.Source) {
		t.Fatal("unpublished target misreported")
	}
}
