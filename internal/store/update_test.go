package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wrk/internal/project"
)

const testID = "wrk-12345678"
const testSource = "---\nid: wrk-12345678\ntitle: Old\nstatus: todo\nfields: {huge: !!int 123456789012345678901234567890}\n---\r\nExact body 🦊\r\n---\nno newline"

func testProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".wrk"), 0755); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{project.ConfigPath: project.DefaultConfig, ".wrk/" + testID + ".md": testSource} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
func TestUpdatePreservesAndNoop(t *testing.T) {
	root := testProject(t)
	status, title := "done", " New 🦊 "
	result := Update(root, testID, &title, &status)
	if len(result.Diagnostics) > 0 || !result.Changed {
		t.Fatal(result.Diagnostics)
	}
	s := project.Load(root)
	if len(s.Diagnostics) > 0 {
		t.Fatal(s.Diagnostics)
	}
	current := s.ByID[testID]
	if current.Title != title || current.Status != status || string(current.Body) != "Exact body 🦊\r\n---\nno newline" {
		t.Fatal(current)
	}
	info, _ := os.Stat(filepath.Join(root, current.Path))
	if info.Mode().Perm() != 0600 {
		t.Fatal("permissions widened")
	}
	result = Update(root, testID, &title, &status)
	if len(result.Diagnostics) > 0 || result.Changed || result.Committed {
		t.Fatal(result)
	}
	after, _ := os.Stat(filepath.Join(root, current.Path))
	if !os.SameFile(info, after) {
		t.Fatal("no-op replaced inode")
	}
	for _, st := range []string{"in-progress", "canceled", "todo"} {
		result = Update(root, testID, nil, &st)
		if len(result.Diagnostics) > 0 {
			t.Fatal(result.Diagnostics)
		}
	}
}
func TestMutationConflicts(t *testing.T) {
	for _, kind := range []string{"target", "config", "inventory", "replacement"} {
		t.Run(kind, func(t *testing.T) {
			root := testProject(t)
			path := filepath.Join(root, ".wrk", testID+".md")
			status := "done"
			h := &hooks{at: func(point string) error {
				if point != "before_compare" {
					return nil
				}
				switch kind {
				case "target":
					info, _ := os.Stat(path)
					os.WriteFile(path, bytes.Replace([]byte(testSource), []byte("Old"), []byte("New"), 1), 0600)
					os.Chtimes(path, info.ModTime(), info.ModTime())
				case "config":
					os.WriteFile(filepath.Join(root, project.ConfigPath), []byte(strings.Replace(project.DefaultConfig, "normal", "urgent", 1)), 0600)
				case "inventory":
					os.WriteFile(filepath.Join(root, ".wrk/wrk-87654321.md"), []byte(strings.Replace(testSource, testID, "wrk-87654321", 1)), 0600)
				case "replacement":
					temp := filepath.Join(root, ".wrk/.wrk-stage-editor")
					os.WriteFile(temp, []byte(testSource), 0600)
					os.Rename(temp, path)
				}
				return nil
			}}
			result := update(root, testID, nil, &status, h)
			if result.Committed || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "CONFLICT" {
				t.Fatalf("%+v", result)
			}
			data, _ := os.ReadFile(path)
			if bytes.Contains(data, []byte("status: done")) {
				t.Fatal("overwrote new state")
			}
		})
	}
}
func TestUpdateInvalidAndPublicationError(t *testing.T) {
	root := testProject(t)
	title := "\n"
	r := Update(root, testID, &title, nil)
	if len(r.Diagnostics) == 0 || r.Committed {
		t.Fatal(r)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".wrk", testID+".md"))
	if string(data) != testSource {
		t.Fatal("invalid mutation changed data")
	}
	status := "done"
	r = update(root, testID, nil, &status, &hooks{at: func(point string) error {
		if point == "dir_sync" {
			return errors.New("injected sync failure")
		}
		return nil
	}})
	if !r.Committed || !r.Changed || r.Diagnostics[0].Code != "DURABILITY_UNCERTAIN" {
		t.Fatal(r)
	}
}
func TestAcceptedEditorRace(t *testing.T) {
	root := testProject(t)
	status := "done"
	path := filepath.Join(root, ".wrk", testID+".md")
	r := update(root, testID, nil, &status, &hooks{at: func(point string) error {
		if point == "after_compare" {
			return os.WriteFile(path, []byte(strings.Replace(testSource, "Exact body", "External body", 1)), 0600)
		}
		return nil
	}})
	if len(r.Diagnostics) > 0 || !r.Committed {
		t.Fatal(r.Diagnostics)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "External body") {
		t.Fatal("test no longer characterizes documented unsupported window")
	}
}

func TestBlockedUpdateAndNoCascade(t *testing.T) {
	root := testProject(t)
	otherID := "wrk-87654321"
	otherPath := filepath.Join(root, ".wrk", otherID+".md")
	other := []byte(strings.Replace(strings.Replace(testSource, testID, otherID, 1), "status: todo", "status: canceled", 1))
	os.WriteFile(otherPath, other, 0600)
	path := filepath.Join(root, ".wrk", testID+".md")
	os.WriteFile(path, []byte(strings.Replace(testSource, "status: todo", "status: todo\ndepends_on: ["+otherID+"]", 1)), 0600)
	status := "done"
	r := Update(root, testID, nil, &status)
	if len(r.Diagnostics) > 0 {
		t.Fatal(r.Diagnostics)
	}
	got, _ := os.ReadFile(otherPath)
	if !bytes.Equal(got, other) {
		t.Fatal("cascade changed prerequisite")
	}
}
func TestPrepublicationFailuresPreserveAllInputs(t *testing.T) {
	for _, point := range []string{"write", "sync", "close", "rename"} {
		t.Run(point, func(t *testing.T) {
			root := testProject(t)
			before := project.Load(root)
			status := "done"
			result := update(root, testID, nil, &status, &hooks{at: func(p string) error {
				if p == point {
					return errors.New("injected")
				}
				return nil
			}})
			if result.Committed || len(result.Diagnostics) == 0 {
				t.Fatal(result)
			}
			for path, original := range before.Files {
				data, _ := os.ReadFile(filepath.Join(root, path))
				if !bytes.Equal(data, original.Data) {
					t.Fatalf("%s changed", path)
				}
			}
		})
	}
}
