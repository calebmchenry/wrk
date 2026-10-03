package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"wrk/internal/project"
	"wrk/internal/schema"
	"wrk/internal/ticket"
)

func TestMetadataUpdatesPreserveAndIgnoreCreationDefaults(t *testing.T) {
	root := testProject(t)
	config := strings.ReplaceAll(strings.Replace(project.DefaultConfig, "normal", "urgent", 1), "labels: []", "labels: [configured]")
	if err := os.WriteFile(filepath.Join(root, project.ConfigPath), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".wrk", testID+".md")
	before := project.Load(root)
	normal := "normal"
	m := UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{Priority: &normal, LabelsSet: true}})
	info, _ := os.Stat(path)
	if len(m.Diagnostics) > 0 || m.Changed || !os.SameFile(before.Files[m.Ticket.Path].Info, info) {
		t.Fatalf("%+v", m)
	}
	title, status, priority := "Combined", "blocked", "high"
	opts := UpdateOptions{Changes: ticket.Changes{Title: &title, Status: &status, Priority: &priority, LabelsSet: true, Labels: []string{"z", "a", "a"}}}
	m = UpdateWithOptions(root, testID, opts)
	if len(m.Diagnostics) > 0 || !m.Committed || m.Ticket.Priority != priority || !slices.Equal(m.Ticket.Labels, opts.Labels) {
		t.Fatalf("%+v", m)
	}
	info, _ = os.Stat(path)
	if info.Mode().Perm() != 0600 || !bytes.Equal(before.ByID[testID].Body, m.Ticket.Body) || !ticket.Equal(schema.Map(before.ByID[testID].Node)["fields"], schema.Map(m.Ticket.Node)["fields"]) {
		t.Fatal("preservation failed")
	}
	for _, priority = range []string{"low", "normal", "urgent"} {
		m = UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{Priority: &priority}})
		if len(m.Diagnostics) > 0 || m.Ticket.Priority != priority || !slices.Equal(m.Ticket.Labels, opts.Labels) {
			t.Fatal(m.Diagnostics)
		}
	}
	m = UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{LabelsSet: true}})
	if len(m.Diagnostics) > 0 || !m.Changed || len(m.Ticket.Labels) != 0 || schema.Map(m.Ticket.Node)["labels"] == nil {
		t.Fatalf("%+v", m)
	}
	info, _ = os.Stat(path)
	source, _ := os.ReadFile(path)
	m = UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{LabelsSet: true}})
	after, _ := os.Stat(path)
	if len(m.Diagnostics) > 0 || m.Changed || !os.SameFile(info, after) {
		t.Fatal("clear no-op rewrote", m.Diagnostics)
	}
	if err := os.WriteFile(filepath.Join(root, project.ConfigPath), []byte(strings.Replace(config, "urgent", "low", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !bytes.Equal(source, data) {
		t.Fatal("config edit changed ticket")
	}
	title = "After defaults change"
	m = Update(root, testID, &title, nil)
	if len(m.Diagnostics) > 0 || m.Ticket.Priority != "urgent" || len(m.Ticket.Labels) != 0 {
		t.Fatal("existing ticket adopted new defaults", m.Diagnostics)
	}
	created := Create(root, CreateOptions{Title: "New defaults"})
	if len(created.Diagnostics) > 0 || created.Ticket.Priority != "low" || !slices.Equal(created.Ticket.Labels, []string{"configured"}) {
		t.Fatal("new ticket did not adopt defaults", created.Diagnostics)
	}
}

func TestInvalidPriorityIsAtomicWithOtherMetadata(t *testing.T) {
	for _, priority := range []string{"", "HIGH", "high ", "invalid"} {
		root := testProject(t)
		title := "Must not publish"
		m := UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{Title: &title, Priority: &priority, LabelsSet: true, Labels: []string{"replacement"}}})
		data, _ := os.ReadFile(filepath.Join(root, ".wrk", testID+".md"))
		if m.Committed || len(m.Diagnostics) == 0 || m.Diagnostics[0].Code != "INVALID_TICKET" || string(data) != testSource {
			t.Fatalf("%q: %+v", priority, m)
		}
	}
}

func TestMetadataPublicationFailures(t *testing.T) {
	for _, point := range []string{"write", "sync", "close", "rename", "dir_sync", "before_compare"} {
		t.Run(point, func(t *testing.T) {
			root := testProject(t)
			related := Create(root, CreateOptions{Title: "Related"})
			if len(related.Diagnostics) > 0 {
				t.Fatal(related.Diagnostics)
			}
			priority := "high"
			opts := UpdateOptions{Changes: ticket.Changes{Priority: &priority, LabelsSet: true, Labels: []string{"new"},
				Parent: &related.Ticket.ID, AddDependencies: []string{related.Ticket.ID}, Fields: customFields(t, "new=!tag [&loop [*loop], !!int 123456789012345678901234567890]")}}
			m := updateWithOptions(root, testID, opts, &hooks{at: func(p string) error {
				if p != point {
					return nil
				}
				if p == "before_compare" {
					return os.WriteFile(filepath.Join(root, project.ConfigPath), []byte(project.DefaultConfig+"# external edit\n"), 0600)
				}
				return errors.New("injected")
			}})
			if len(m.Diagnostics) == 0 || m.Committed != (point == "dir_sync") {
				t.Fatalf("%+v", m)
			}
			data, _ := os.ReadFile(filepath.Join(root, ".wrk", testID+".md"))
			if point == "dir_sync" {
				if m.Diagnostics[0].Code != "DURABILITY_UNCERTAIN" || m.Ticket.Priority != priority || !slices.Equal(m.Ticket.Labels, opts.Labels) {
					t.Fatalf("%+v", m)
				}
			} else if string(data) != testSource {
				t.Fatal("pre-publication mutation")
			}
			if point == "before_compare" && m.Diagnostics[0].Code != "CONFLICT" {
				t.Fatal(m.Diagnostics)
			}
			retry := UpdateWithOptions(root, testID, opts)
			if len(retry.Diagnostics) > 0 || retry.Changed == (point == "dir_sync") {
				t.Fatalf("retry: %+v", retry)
			}
		})
	}
}

func TestRecursiveReplacementAndClear(t *testing.T) {
	root := recursiveProject(t)
	unrelated, _ := os.ReadFile(filepath.Join(root, ".wrk/wrk-ffffffff.md"))
	for _, labels := range [][]string{{"z", "a", "a"}, {}} {
		opts := UpdateOptions{Changes: ticket.Changes{LabelsSet: true, Labels: labels}, Recursive: true}
		m := UpdateWithOptions(root, testID, opts)
		if len(m.Diagnostics) > 0 || len(m.Updates) != 5 || !m.Changed {
			t.Fatalf("%+v", m)
		}
		for _, entry := range m.Updates {
			if !slices.Equal(entry.Ticket.Labels, labels) {
				t.Fatal(entry.Ticket.Labels)
			}
		}
		retry := UpdateWithOptions(root, testID, opts)
		if len(retry.Diagnostics) > 0 || retry.Changed {
			t.Fatalf("%+v", retry)
		}
	}
	got, _ := os.ReadFile(filepath.Join(root, ".wrk/wrk-ffffffff.md"))
	if !bytes.Equal(got, unrelated) {
		t.Fatal("changed unrelated ticket")
	}
	priority := "normal"
	before := project.Load(root)
	m := UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{Priority: &priority, LabelsSet: true}, Recursive: true})
	if m.Committed || len(m.Diagnostics) != 1 || m.Diagnostics[0].Code != "USAGE" {
		t.Fatalf("%+v", m)
	}
	for path, old := range before.Files {
		data, _ := os.ReadFile(filepath.Join(root, path))
		if !bytes.Equal(data, old.Data) {
			t.Fatal("recursive priority touched", path)
		}
	}
}
