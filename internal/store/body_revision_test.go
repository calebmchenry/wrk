package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wrk/internal/project"
	"wrk/internal/ticket"
)

func requireMutationCode(t *testing.T, m Mutation, code string) {
	t.Helper()
	found := false
	for _, d := range m.Diagnostics {
		found = found || d.Code == code
	}
	if m.Committed || m.Changed || !found {
		t.Fatalf("want %s without publication: %+v", code, m)
	}
}

func TestRevisionRejectsPriorChangesAndDeletion(t *testing.T) {
	for _, kind := range []string{"metadata", "body", "comment", "invalid", "deleted", "deleted referenced", "nonregular"} {
		t.Run(kind, func(t *testing.T) {
			root := testProject(t)
			path := filepath.Join(root, ".wrk", testID+".md")
			original := project.Load(root)
			revision := original.Summary(original.ByID[testID]).Revision
			var err error
			switch kind {
			case "metadata":
				status := "done"
				if m := Update(root, testID, nil, &status); len(m.Diagnostics) > 0 {
					t.Fatal(m.Diagnostics)
				}
			case "body":
				body := "Agent body"
				if m := UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{Body: &body}}); len(m.Diagnostics) > 0 {
					t.Fatal(m.Diagnostics)
				}
			case "comment":
				err = os.WriteFile(path, []byte(strings.Replace(testSource, "title: Old", "title: Old # external edit", 1)), 0600)
			case "invalid":
				err = os.WriteFile(path, []byte("---\nmalformed: [\n---\n"), 0600)
			case "deleted", "deleted referenced":
				if kind == "deleted referenced" {
					parent := testID
					if m := Create(root, CreateOptions{Title: "Child", Parent: &parent}); len(m.Diagnostics) > 0 {
						t.Fatal(m.Diagnostics)
					}
				}
				err = os.Remove(path)
			case "nonregular":
				if err = os.Remove(path); err == nil {
					err = os.Mkdir(path, 0700)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			before := project.Load(root)
			want := "CONFLICT"
			if strings.HasPrefix(kind, "deleted") {
				want = "NOT_FOUND"
			}
			// Both a real edit and an edit already equal to current data must fail.
			for _, body := range []string{"Browser draft", "Agent body"} {
				m := UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{Body: &body}, ExpectedRevision: &revision})
				requireMutationCode(t, m, want)
				if err := before.Compare(); err != nil {
					t.Fatal("rejected edit changed project:", err)
				}
			}
		})
	}
}

func TestRevisionReloadRetryAndNoop(t *testing.T) {
	root := testProject(t)
	s := project.Load(root)
	path := filepath.Join(root, s.ByID[testID].Path)
	original := s.Summary(s.ByID[testID]).Revision
	status := "in-progress"
	if m := Update(root, testID, nil, &status); len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	body := "Reviewed draft\r\nno newline 🦊"
	opts := UpdateOptions{Changes: ticket.Changes{Body: &body}, ExpectedRevision: &original}
	requireMutationCode(t, UpdateWithOptions(root, testID, opts), "CONFLICT")
	s = project.Load(root)
	current := s.Summary(s.ByID[testID]).Revision
	opts.ExpectedRevision = &current
	m := UpdateWithOptions(root, testID, opts)
	if len(m.Diagnostics) > 0 || !m.Committed || string(m.Ticket.Body) != body || m.Ticket.Status != status {
		t.Fatal(m)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("permissions changed", err)
	}
	next := m.Snapshot.Summary(m.Ticket).Revision
	reloaded := project.Load(root)
	if next == current || next == original || next != reloaded.Summary(reloaded.ByID[testID]).Revision {
		t.Fatal("incorrect committed revision")
	}
	// Identical requested body does not bypass an obsolete precondition.
	requireMutationCode(t, UpdateWithOptions(root, testID, opts), "CONFLICT")
	opts.ExpectedRevision = &next
	m = UpdateWithOptions(root, testID, opts)
	if len(m.Diagnostics) > 0 || m.Changed || m.Committed {
		t.Fatal(m)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(info, after) {
		t.Fatal("no-op replaced file", err)
	}
}

func TestRevisionScopeAndCurrentValidation(t *testing.T) {
	root := testProject(t)
	s := project.Load(root)
	revision := s.Summary(s.ByID[testID]).Revision
	if m := Create(root, CreateOptions{Title: "Unrelated"}); len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	// A same-byte replacement and mode/config changes are outside the revision.
	path := filepath.Join(root, s.ByID[testID].Path)
	replacement := filepath.Join(root, ".wrk/.wrk-stage-external")
	if err := os.WriteFile(replacement, []byte(testSource), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, project.ConfigPath)
	if err := os.WriteFile(configPath, []byte(strings.Replace(project.DefaultConfig, "normal", "urgent", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	body := "Updated"
	opts := UpdateOptions{Changes: ticket.Changes{Body: &body}, ExpectedRevision: &revision}
	m := UpdateWithOptions(root, testID, opts)
	if len(m.Diagnostics) > 0 || !m.Committed || m.Ticket.Priority != "normal" {
		t.Fatal(m)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatal("current mode not preserved", err)
	}
	revision = m.Snapshot.Summary(m.Ticket).Revision
	// A valid current revision cannot bypass new configuration or graph checks.
	if err := os.WriteFile(configPath, []byte("version: 1\nprefix: wrk\nfields: {estimate: {type: number}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	field, err := ticket.ParseField("estimate=wrong")
	if err != nil {
		t.Fatal(err)
	}
	opts.Fields = []ticket.FieldValue{field}
	requireMutationCode(t, UpdateWithOptions(root, testID, opts), "INVALID_TICKET")
	opts.Fields = nil
	opts.AddDependencies = []string{"wrk-deadbeef"}
	requireMutationCode(t, UpdateWithOptions(root, testID, opts), "MISSING_REFERENCE")
	opts.AddDependencies = nil
	if err := os.WriteFile(configPath, []byte("invalid: ["), 0600); err != nil {
		t.Fatal(err)
	}
	requireMutationCode(t, UpdateWithOptions(root, testID, opts), "INVALID_CONFIG")
}

func TestBodyRevisionGuardsAndPublication(t *testing.T) {
	root := testProject(t)
	body, emptyRevision := "New body", ""
	opts := UpdateOptions{Changes: ticket.Changes{Body: &body}, ExpectedRevision: &emptyRevision}
	requireMutationCode(t, UpdateWithOptions(root, testID, opts), "CONFLICT")
	lock, err := Acquire(filepath.Join(root, ".wrk"))
	if err != nil {
		t.Fatal(err)
	}
	// Lock acquisition precedes revision inspection.
	requireMutationCode(t, UpdateWithOptions(root, testID, opts), "BUSY")
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	opts.Recursive = true
	requireMutationCode(t, UpdateWithOptions(root, testID, opts), "USAGE")
	opts.Changes = ticket.Changes{AddLabels: []string{"x"}}
	requireMutationCode(t, UpdateWithOptions(root, testID, opts), "USAGE")
	invalid := "\xff"
	requireMutationCode(t, UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{Body: &invalid}}), "INVALID_BODY")
	for _, point := range []string{"write", "sync", "close", "rename", "dir_sync"} {
		t.Run(point, func(t *testing.T) {
			root := testProject(t)
			s := project.Load(root)
			revision := s.Summary(s.ByID[testID]).Revision
			m := updateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{Body: &body}, ExpectedRevision: &revision}, &hooks{at: func(p string) error {
				if p == point {
					return errors.New("injected failure")
				}
				return nil
			}})
			if point == "dir_sync" {
				if !m.Committed || !m.Changed || len(m.Diagnostics) != 1 || m.Diagnostics[0].Code != "DURABILITY_UNCERTAIN" || string(project.Load(root).ByID[testID].Body) != body {
					t.Fatal(m)
				}
			} else {
				requireMutationCode(t, m, "IO")
				if err := s.Compare(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestRevisionRetainsFinalSnapshotComparison(t *testing.T) {
	for _, kind := range []string{"target", "config", "inventory"} {
		t.Run(kind, func(t *testing.T) {
			root := testProject(t)
			s := project.Load(root)
			revision := s.Summary(s.ByID[testID]).Revision
			body := "Browser draft"
			m := updateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{Body: &body}, ExpectedRevision: &revision}, &hooks{at: func(point string) error {
				if point != "before_compare" {
					return nil
				}
				switch kind {
				case "target":
					return os.WriteFile(filepath.Join(root, ".wrk", testID+".md"), []byte(testSource+"\nExternal edit"), 0600)
				case "config":
					return os.WriteFile(filepath.Join(root, project.ConfigPath), []byte(project.DefaultConfig+"\n# external edit\n"), 0600)
				default:
					return os.WriteFile(filepath.Join(root, ".wrk/wrk-87654321.md"), bytes.ReplaceAll([]byte(testSource), []byte(testID), []byte("wrk-87654321")), 0600)
				}
			}})
			requireMutationCode(t, m, "CONFLICT")
			if string(project.Load(root).ByID[testID].Body) == body {
				t.Fatal("overwrote concurrent edit")
			}
		})
	}
}
