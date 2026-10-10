package store

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"wrk/internal/project"
	"wrk/internal/schema"
	"wrk/internal/ticket"
)

const relatedA = "wrk-00000001"
const relatedB = "wrk-00000002"

func relatedProject(t *testing.T) string {
	t.Helper()
	root := testProject(t)
	for _, id := range []string{relatedA, relatedB} {
		source := strings.ReplaceAll(testSource, testID, id)
		source = strings.Replace(source, "fields:", "related: &links ["+testID+"]\nfields:", 1)
		source = strings.Replace(source, "{huge:", "{links: *links, loop: &loop !tag [*loop], huge:", 1)
		if err := os.WriteFile(filepath.Join(root, ".wrk", id+".md"), []byte(source), 0640); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func requireRelatedMutation(t *testing.T, m Mutation) {
	t.Helper()
	if len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
}

func TestRelatedReciprocityCyclesAndPreservation(t *testing.T) {
	root := relatedProject(t)
	before := project.Load(root)
	if len(before.Diagnostics) > 0 {
		t.Fatal(before.Diagnostics)
	}
	if !slices.Equal(before.Summary(before.ByID[testID]).Related, []string{relatedA, relatedB}) {
		t.Fatal("missing reverse links")
	}
	// Adding from the opposite endpoint never duplicates or moves the edge.
	m := UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{AddRelated: []string{relatedA, relatedA}}})
	requireRelatedMutation(t, m)
	if m.Changed || m.Committed {
		t.Fatal("reverse add rewrote an edge")
	}
	assertProjectUnchanged(t, root, before)
	// All three items may form a contextual cycle. Existing parent/dependency
	// relationships and explicit statuses still determine readiness independently.
	parent := testID
	m = UpdateWithOptions(root, relatedA, UpdateOptions{Changes: ticket.Changes{AddRelated: []string{relatedB}, Parent: &parent, AddDependencies: []string{testID}}})
	requireRelatedMutation(t, m)
	s := project.Load(root)
	if len(s.Diagnostics) > 0 || len(s.RelatedIDs(relatedB)) != 2 || len(s.Blockers(s.ByID[relatedA])) != 1 {
		t.Fatal(s.Diagnostics)
	}
	if len(s.List(false, true)) != 2 || s.ByID[testID].Status != "todo" {
		t.Fatal("related links changed readiness/status")
	}
	before = s
	title := "Combined edit"
	m = UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{NoRelated: true, Title: &title}})
	requireRelatedMutation(t, m)
	if len(m.Updates) != 3 || !m.Changed || len(m.Snapshot.Summary(m.Ticket).Related) != 0 {
		t.Fatal(m)
	}
	after := project.Load(root)
	if !slices.Equal(after.RelatedIDs(relatedA), []string{relatedB}) {
		t.Fatal("unrelated edge lost")
	}
	for path, old := range before.Files {
		now := after.Files[path]
		if path == project.ConfigPath {
			if !bytes.Equal(old.Data, now.Data) {
				t.Fatal("config changed")
			}
			continue
		}
		a, b := before.ByID[strings.TrimSuffix(filepath.Base(path), ".md")], after.ByID[strings.TrimSuffix(filepath.Base(path), ".md")]
		if !bytes.Equal(a.Body, b.Body) || old.Info.Mode() != now.Info.Mode() {
			t.Fatal("body/mode changed", path)
		}
		for key, value := range schema.Map(a.Node) {
			if key == "related" || (a.ID == testID && key == "title") {
				continue
			}
			if !ticket.Equal(value, schema.Map(b.Node)[key]) {
				t.Fatal("unrelated YAML changed", key)
			}
		}
	}
	m = UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{NoRelated: true}})
	requireRelatedMutation(t, m)
	if m.Changed {
		t.Fatal("clear not idempotent")
	}
	assertProjectUnchanged(t, root, after)
	// New tickets store links locally and immediately expose their summary.
	m = Create(root, CreateOptions{Title: "Created related", Related: []string{relatedA, relatedB}})
	requireRelatedMutation(t, m)
	if !slices.Equal(m.Snapshot.Summary(m.Ticket).Related, []string{relatedA, relatedB}) {
		t.Fatal("create summary")
	}
	if !slices.Contains(project.Load(root).RelatedIDs(relatedA), m.Ticket.ID) {
		t.Fatal("create reverse view")
	}
}

func TestRelatedValidationAndNoops(t *testing.T) {
	root := relatedProject(t)
	before := project.Load(root)
	for _, c := range []ticket.Changes{
		{AddRelated: []string{testID}}, {RemoveRelated: []string{testID}},
		{AddRelated: []string{"wrk-deadbeef"}}, {RemoveRelated: []string{"wrk-deadbeef"}},
		{AddRelated: []string{"bad"}}, {RemoveRelated: []string{"bad"}},
		{AddRelated: []string{relatedA}, RemoveRelated: []string{relatedA}},
		{NoRelated: true, AddRelated: []string{relatedA}}, {NoRelated: true, RemoveRelated: []string{relatedA}},
	} {
		title := "Must not publish"
		c.Title = &title
		m := UpdateWithOptions(root, testID, UpdateOptions{Changes: c})
		if len(m.Diagnostics) == 0 || m.Committed {
			t.Fatal("invalid related edit accepted", c)
		}
		assertProjectUnchanged(t, root, before)
	}
	for _, refs := range [][]string{{relatedA, relatedA}, {"wrk-deadbeef"}, {"bad"}} {
		m := Create(root, CreateOptions{Title: "Invalid", Related: refs})
		if len(m.Diagnostics) == 0 || m.Committed {
			t.Fatal("invalid related create accepted", refs)
		}
		assertProjectUnchanged(t, root, before)
	}
	m := UpdateWithOptions(root, relatedA, UpdateOptions{Changes: ticket.Changes{RemoveRelated: []string{relatedB, relatedB}}})
	requireRelatedMutation(t, m)
	if m.Changed {
		t.Fatal("absent edge removal changed files")
	}
	assertProjectUnchanged(t, root, before)
	// Validate rejects duplicated pairs from manual edits/merges, in either direction.
	for _, refs := range []string{"[" + relatedA + "]", "[" + relatedA + ", " + relatedA + "]", "null", "123", "[" + testID + "]"} {
		source := strings.Replace(testSource, "fields:", "related: "+refs+"\nfields:", 1)
		if err := os.WriteFile(filepath.Join(root, ".wrk", testID+".md"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		if len(project.Load(root).Diagnostics) == 0 {
			t.Fatal("invalid stored links accepted", refs)
		}
	}
}

func TestRelatedRevisions(t *testing.T) {
	root := relatedProject(t)
	s := project.Load(root)
	base, related := s.Summary(s.ByID[testID]).Revision, s.RelatedRevision(testID)
	requireRelatedMutation(t, UpdateWithOptions(root, relatedA, UpdateOptions{Changes: ticket.Changes{RemoveRelated: []string{testID}}}))
	current := project.Load(root)
	if current.Summary(current.ByID[testID]).Revision != base || current.RelatedRevision(testID) == related {
		t.Fatal("revision scopes are wrong")
	}
	for _, c := range []ticket.Changes{{NoRelated: true}, {AddRelated: []string{relatedB}}, {Title: &current.ByID[testID].Title}} {
		m := UpdateWithOptions(root, testID, UpdateOptions{Changes: c, ExpectedRevision: &base, ExpectedRelatedRevision: &related})
		if m.Committed || len(m.Diagnostics) != 1 || m.Diagnostics[0].Code != "CONFLICT" {
			t.Fatal(m)
		}
		assertProjectUnchanged(t, root, current)
	}
	related = current.RelatedRevision(testID)
	// Changes to an owner's unrelated values are preserved, not overwritten.
	title := "Owner title"
	requireRelatedMutation(t, Update(root, relatedB, &title, nil))
	m := UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{RemoveRelated: []string{relatedB}}, ExpectedRevision: &base, ExpectedRelatedRevision: &related})
	requireRelatedMutation(t, m)
	if project.Load(root).ByID[relatedB].Title != title {
		t.Fatal("owner metadata lost")
	}
}

func TestRelatedBatchFailuresAndRetry(t *testing.T) {
	for _, tc := range []struct {
		point                 string
		occurrence, committed int
	}{
		{"write", 2, 0}, {"sync", 2, 0}, {"before_compare", 1, 0}, {"rename", 1, 0}, {"rename", 2, 1}, {"dir_sync", 2, 2},
	} {
		t.Run(fmt.Sprintf("%s-%d", tc.point, tc.occurrence), func(t *testing.T) {
			root := relatedProject(t)
			title := "Combined"
			opts := UpdateOptions{Changes: ticket.Changes{NoRelated: true, Title: &title}}
			hits := 0
			m := updateWithOptions(root, testID, opts, &hooks{at: func(point string) error {
				if point == tc.point {
					hits++
					if hits == tc.occurrence {
						return errors.New("injected")
					}
				}
				return nil
			}})
			if len(m.Diagnostics) == 0 || m.Committed != (tc.committed > 0) || len(m.Updates) != 3 {
				t.Fatal(m)
			}
			for i, e := range m.Updates {
				want := "pending"
				if i < tc.committed {
					want = "committed"
				}
				if e.Publication != want {
					t.Fatal(e, want)
				}
			}
			if s := project.Load(root); len(s.Diagnostics) > 0 {
				t.Fatal("partial publication invalid", s.Diagnostics)
			}
			requireRelatedMutation(t, UpdateWithOptions(root, testID, opts))
			if s := project.Load(root); len(s.RelatedIDs(testID)) != 0 || s.ByID[testID].Title != title {
				t.Fatal("retry incomplete")
			}
		})
	}
}

func TestRelatedLockConflictAndPreservationFailure(t *testing.T) {
	for _, mode := range []string{"external", "preservation"} {
		t.Run(mode, func(t *testing.T) {
			root := relatedProject(t)
			var h *hooks
			if mode == "preservation" {
				source := "---\n&whole\nid: " + relatedB + "\ntitle: Alias\nstatus: todo\nrelated: [" + testID + "]\nfields: {whole: *whole}\n---\nbody"
				if err := os.WriteFile(filepath.Join(root, ".wrk", relatedB+".md"), []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				h = &hooks{at: func(point string) error {
					if point != "before_compare" {
						return nil
					}
					lock, err := Acquire(filepath.Join(root, ".wrk"))
					if err == nil {
						lock.Close()
						t.Fatal("writer lock released")
					}
					if !errors.Is(err, ErrBusy) {
						t.Fatal(err)
					}
					return os.WriteFile(filepath.Join(root, ".wrk", testID+".md"), []byte(testSource+" external"), 0600)
				}}
			}
			before := project.Load(root)
			m := updateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{NoRelated: true}}, h)
			want := "CONFLICT"
			if mode == "preservation" {
				want = "PRESERVATION_UNSUPPORTED"
				assertProjectUnchanged(t, root, before)
			}
			if m.Committed || len(m.Diagnostics) == 0 || m.Diagnostics[0].Code != want {
				t.Fatal(m)
			}
			for _, id := range []string{relatedA, relatedB} {
				got, err := os.ReadFile(filepath.Join(root, before.ByID[id].Path))
				if err != nil || !bytes.Equal(got, before.ByID[id].Source) {
					t.Fatal("owner changed on failure")
				}
			}
		})
	}
}
