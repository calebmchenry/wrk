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

func recursiveProject(t *testing.T) string {
	t.Helper()
	root := testProject(t)
	// IDs put the root after one child: results/publication must sort by ID.
	for i, status := range []string{"blocked", "in-progress", "done", "canceled"} {
		id := fmt.Sprintf("wrk-%08x", i+1)
		parent := testID
		if i > 0 {
			parent = fmt.Sprintf("wrk-%08x", i)
		}
		source := fmt.Sprintf("---\nid: %s\ntitle: Child\nstatus: %s\nparent: %s\nlabels: &labels [&label keep, keep]\nfields: {labels: *labels, label: *label, opaque: !tag [1, 2], loop: &loop [*loop]}\n---\r\nBody 🦊\r\nno newline", id, status, parent)
		if err := os.WriteFile(filepath.Join(root, ".wrk", id+".md"), []byte(source), 0640); err != nil {
			t.Fatal(err)
		}
	}
	// A dependency is not a descendant; an unrelated dependent is not one either.
	source := strings.ReplaceAll(testSource, testID, "wrk-ffffffff")
	source = strings.Replace(source, "status: todo", "status: todo\ndepends_on: ["+testID+"]", 1)
	if err := os.WriteFile(filepath.Join(root, ".wrk/wrk-ffffffff.md"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".wrk", testID+".md")
	if err := os.WriteFile(path, []byte(strings.Replace(testSource, "status: todo", "status: todo\ndepends_on: [wrk-00000004]", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}
func burnOptions() UpdateOptions {
	return UpdateOptions{Changes: ticket.Changes{AddLabels: []string{"burn", "burn"}, RemoveLabels: []string{"absent"}}, Recursive: true}
}

func TestRecursivePreservationNoopsAndSnapshot(t *testing.T) {
	root := recursiveProject(t)
	before := project.Load(root)
	m := UpdateWithOptions(root, testID, burnOptions())
	if len(m.Diagnostics) > 0 || len(m.Updates) != 5 || !m.Changed {
		t.Fatalf("%+v", m)
	}
	after := project.Load(root)
	for i, e := range m.Updates {
		if e.Publication != "committed" || (i > 0 && e.Ticket.ID < m.Updates[i-1].Ticket.ID) {
			t.Fatalf("%+v", m.Updates)
		}
	}
	for path, old := range before.Files {
		now := after.Files[path]
		if path == project.ConfigPath || strings.Contains(path, "ffffffff") {
			if !bytes.Equal(now.Data, old.Data) || !os.SameFile(now.Info, old.Info) {
				t.Fatal("unrelated input changed", path)
			}
			continue
		}
		oldTicket, _ := ticket.Parse(old.Data, path)
		newTicket, _ := ticket.Parse(now.Data, path)
		if !bytes.Equal(oldTicket.Body, newTicket.Body) || old.Info.Mode() != now.Info.Mode() {
			t.Fatal("body/mode changed", path)
		}
		for key, node := range schema.Map(oldTicket.Node) {
			if key != "labels" && !ticket.Equal(node, schema.Map(newTicket.Node)[key]) {
				t.Fatal("unrelated metadata changed", path, key)
			}
		}
	}
	m = UpdateWithOptions(root, testID, burnOptions())
	if len(m.Diagnostics) > 0 || m.Changed || m.Committed {
		t.Fatalf("%+v", m)
	}
	for path, old := range after.Files {
		now, _ := os.Stat(filepath.Join(root, path))
		if !os.SameFile(old.Info, now) {
			t.Fatal("no-op rewrote", path)
		}
	}
	for _, e := range m.Updates {
		if e.Publication != "unchanged" {
			t.Fatal(e)
		}
	}
	parentID := testID
	child := Create(root, CreateOptions{Title: "Later", Parent: &parentID})
	if len(child.Diagnostics) > 0 || len(child.Ticket.Labels) > 0 {
		t.Fatal("labels inherited", child.Diagnostics)
	}
	opts := UpdateOptions{Changes: ticket.Changes{RemoveLabels: []string{"burn"}}, Recursive: true}
	m = UpdateWithOptions(root, testID, opts)
	if len(m.Diagnostics) > 0 || len(m.Updates) != 6 {
		t.Fatalf("%+v", m)
	}
	for _, e := range m.Updates {
		if slices.Contains(e.Ticket.Labels, "burn") {
			t.Fatal("not removed")
		}
	}
}

func TestRecursiveStagingAndPublicationFailures(t *testing.T) {
	for _, tc := range []struct {
		point                 string
		occurrence, committed int
	}{
		{"write", 3, 0}, {"sync", 3, 0}, {"close", 3, 0}, {"before_compare", 1, 0}, {"rename", 1, 0}, {"rename", 3, 2}, {"dir_sync", 2, 2},
	} {
		t.Run(fmt.Sprintf("%s-%d", tc.point, tc.occurrence), func(t *testing.T) {
			root := recursiveProject(t)
			before := project.Load(root)
			hits := 0
			m := updateWithOptions(root, testID, burnOptions(), &hooks{at: func(point string) error {
				if point == tc.point {
					hits++
					if hits == tc.occurrence {
						return errors.New("injected")
					}
				}
				return nil
			}})
			if len(m.Diagnostics) == 0 || m.Committed != (tc.committed > 0) {
				t.Fatalf("%+v", m)
			}
			for i, e := range m.Updates {
				want := "pending"
				if i < tc.committed {
					want = "committed"
				}
				if e.Publication != want {
					t.Fatalf("%d: %s want %s", i, e.Publication, want)
				}
				data, _ := os.ReadFile(filepath.Join(root, e.Ticket.Path))
				if (i < tc.committed) == bytes.Equal(data, before.Files[e.Ticket.Path].Data) {
					t.Fatalf("unexpected publication %s", e.Ticket.ID)
				}
			}
			stages, _ := filepath.Glob(filepath.Join(root, ".wrk/.wrk-stage-*"))
			if len(stages) != 0 {
				t.Fatal("leaked stages", stages)
			}
			retry := UpdateWithOptions(root, testID, burnOptions())
			if len(retry.Diagnostics) > 0 {
				t.Fatal(retry.Diagnostics)
			}
			for i, e := range retry.Updates {
				if i < tc.committed && e.Publication != "unchanged" {
					t.Fatal("retry rewrote committed ticket")
				}
			}
		})
	}
}

func TestRecursiveStaleInputsAndLock(t *testing.T) {
	for _, kind := range []string{"config", "unrelated", "pending", "committed", "inode", "mode", "inventory"} {
		t.Run(kind, func(t *testing.T) {
			root := recursiveProject(t)
			count := 0
			m := updateWithOptions(root, testID, burnOptions(), &hooks{at: func(point string) error {
				if point != "before_compare" {
					return nil
				}
				count++
				lock, err := Acquire(filepath.Join(root, ".wrk"))
				if err == nil {
					lock.Close()
					t.Fatal("batch released lock")
				}
				if !errors.Is(err, ErrBusy) {
					t.Fatal(err)
				}
				if count != 2 {
					return nil
				}
				path := filepath.Join(root, ".wrk/wrk-ffffffff.md")
				switch kind {
				case "config":
					path = filepath.Join(root, project.ConfigPath)
				case "pending":
					path = filepath.Join(root, ".wrk/wrk-00000002.md")
				case "committed", "inode", "mode":
					path = filepath.Join(root, ".wrk/wrk-00000001.md")
				case "inventory":
					return os.WriteFile(filepath.Join(root, ".wrk/wrk-eeeeeeee.md"), []byte(strings.ReplaceAll(testSource, testID, "wrk-eeeeeeee")), 0600)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if kind == "mode" {
					return os.Chmod(path, 0400)
				}
				if kind == "inode" {
					temp := filepath.Join(root, ".wrk/.wrk-stage-editor")
					if err := os.WriteFile(temp, data, 0640); err != nil {
						return err
					}
					return os.Rename(temp, path)
				}
				return os.WriteFile(path, append(data, []byte("\n# external edit\n")...), 0600)
			}})
			if !m.Committed || len(m.Diagnostics) != 1 || m.Diagnostics[0].Code != "CONFLICT" || m.Updates[0].Publication != "committed" || m.Updates[1].Publication != "pending" {
				t.Fatalf("%+v", m)
			}
		})
	}
}

func TestRecursiveCleanupFailure(t *testing.T) {
	root := recursiveProject(t)
	hits := 0
	m := updateWithOptions(root, testID, burnOptions(), &hooks{at: func(point string) error {
		if point == "rename" {
			hits++
			if hits == 2 {
				return errors.New("rename failed")
			}
		}
		if point == "cleanup" {
			return errors.New("cleanup failed")
		}
		return nil
	}})
	if !m.Committed || len(m.Diagnostics) < 2 {
		t.Fatalf("%+v", m)
	}
	stages, _ := filepath.Glob(filepath.Join(root, ".wrk/.wrk-stage-*"))
	if len(stages) != 4 {
		t.Fatal(stages)
	}
	if s := project.Load(root); len(s.Diagnostics) > 0 {
		t.Fatal(s.Diagnostics)
	}
	if retry := UpdateWithOptions(root, testID, burnOptions()); len(retry.Diagnostics) > 0 {
		t.Fatal(retry.Diagnostics)
	}
}

func TestRecursivePreservationFailureCommitsNothing(t *testing.T) {
	root := recursiveProject(t)
	path := filepath.Join(root, ".wrk/wrk-00000004.md")
	// A custom alias to the entire ticket makes any metadata change also change
	// unrelated custom data. Refuse this otherwise valid graph before publication.
	source := "---\n&whole\nid: wrk-00000004\ntitle: Whole alias\nstatus: todo\nparent: " + testID + "\nfields: {whole: *whole}\n---\nbody"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	before := project.Load(root)
	if len(before.Diagnostics) > 0 {
		t.Fatal(before.Diagnostics)
	}
	m := UpdateWithOptions(root, testID, burnOptions())
	if m.Committed || len(m.Diagnostics) != 1 || m.Diagnostics[0].Code != "PRESERVATION_UNSUPPORTED" {
		t.Fatalf("%+v", m)
	}
	for path, old := range before.Files {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || !bytes.Equal(data, old.Data) {
			t.Fatal("changed despite preservation failure", path, err)
		}
	}
	stages, _ := filepath.Glob(filepath.Join(root, ".wrk/.wrk-stage-*"))
	if len(stages) > 0 {
		t.Fatal("staged before all validation", stages)
	}
}
