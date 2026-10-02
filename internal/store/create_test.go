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

func TestInitAndCreationDefaults(t *testing.T) {
	root := t.TempDir()
	r := Init(root)
	if len(r.Diagnostics) > 0 || !r.Created {
		t.Fatal(r)
	}
	config, _ := os.ReadFile(filepath.Join(root, project.ConfigPath))
	if string(config) != project.DefaultConfig {
		t.Fatal("wrong initial config")
	}
	for _, kind := range []string{"directory", "file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			target := t.TempDir()
			path := filepath.Join(target, ".wrk")
			switch kind {
			case "directory":
				os.Mkdir(path, 0755)
			case "file":
				os.WriteFile(path, []byte("keep"), 0600)
			case "symlink":
				os.Symlink(root, path)
			}
			if r := Init(target); r.Created || len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != "ALREADY_EXISTS" {
				t.Fatal(r)
			}
		})
	}
	os.WriteFile(filepath.Join(root, project.ConfigPath), []byte("version: 1\nprefix: custom\ndefaults: {priority: high, labels: [default]}\n"), 0600)
	first := Create(root, CreateOptions{Title: "First", Body: []byte("\r\n🦊 no newline")})
	if len(first.Diagnostics) > 0 {
		t.Fatal(first.Diagnostics)
	}
	if !strings.HasPrefix(first.Ticket.ID, "custom-") || first.Ticket.Priority != "high" || len(first.Ticket.Labels) != 1 || !bytes.Equal(first.Ticket.Body, []byte("\r\n🦊 no newline")) {
		t.Fatal(first.Ticket)
	}
	priority := "low"
	parent := first.Ticket.ID
	second := Create(root, CreateOptions{Title: "Child", Parent: &parent, Priority: &priority, LabelsSet: true})
	if len(second.Diagnostics) > 0 || second.Ticket.Priority != "low" || len(second.Ticket.Labels) != 0 {
		t.Fatal(second)
	}
	os.WriteFile(filepath.Join(root, project.ConfigPath), []byte("version: 1\nprefix: next\ndefaults: {priority: urgent, labels: [new]}\n"), 0600)
	third := Create(root, CreateOptions{Title: "Third", Labels: []string{"a", "a", "b"}, LabelsSet: true})
	if len(third.Diagnostics) > 0 || !strings.HasPrefix(third.Ticket.ID, "next-") || len(third.Ticket.Labels) != 3 {
		t.Fatal(third)
	}
	s := project.Load(root)
	if len(s.Diagnostics) > 0 || s.ByID[first.Ticket.ID].Priority != "high" || s.ByID[second.Ticket.ID].Priority != "low" {
		t.Fatal(s.Diagnostics)
	}
}
func TestBoundedCollisionAndLateCollision(t *testing.T) {
	root := testProject(t)
	old, _ := os.ReadFile(filepath.Join(root, ".wrk", testID+".md"))
	rng := bytes.NewReader(bytes.Repeat([]byte{0x12, 0x34, 0x56, 0x78}, 128))
	r := create(root, CreateOptions{Title: "New"}, rng, nil)
	if r.Committed || r.Diagnostics[0].Code != "ID_EXHAUSTED" || rng.Len() != 0 {
		t.Fatal(r)
	}
	got, _ := os.ReadFile(filepath.Join(root, ".wrk", testID+".md"))
	if !bytes.Equal(old, got) {
		t.Fatal("clobbered collision")
	}
	calls := 0
	r = create(root, CreateOptions{Title: "New"}, bytes.NewReader([]byte{0, 0, 0, 1, 0, 0, 0, 2}), &hooks{at: func(point string) error {
		if point == "after_compare" {
			calls++
			if calls == 1 {
				return os.WriteFile(filepath.Join(root, ".wrk/wrk-00000001.md"), []byte(strings.Replace(testSource, testID, "wrk-00000001", 1)), 0600)
			}
		}
		return nil
	}})
	if len(r.Diagnostics) > 0 || !r.Created || r.Ticket.ID != "wrk-00000002" {
		t.Fatal(r)
	}
	got, _ = os.ReadFile(filepath.Join(root, ".wrk/wrk-00000001.md"))
	if !bytes.Contains(got, []byte("title: Old")) {
		t.Fatal("overwrote late collision")
	}
}
func TestInitHandledFailureLeavesForeignFiles(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		root := t.TempDir()
		r := initialize(root, &hooks{at: func(point string) error {
			if point == "write" {
				if foreign {
					os.WriteFile(filepath.Join(root, ".wrk/foreign"), []byte("keep"), 0600)
				}
				return errors.New("write failed")
			}
			return nil
		}})
		if r.Created || len(r.Diagnostics) == 0 {
			t.Fatal(r)
		}
		_, err := os.Stat(filepath.Join(root, ".wrk"))
		if foreign {
			b, err := os.ReadFile(filepath.Join(root, ".wrk/foreign"))
			if err != nil || string(b) != "keep" {
				t.Fatal("removed foreign file")
			}
		} else if !os.IsNotExist(err) {
			t.Fatal("left empty failed init")
		}
	}
}
func TestCreateInvalidDoesNotChangeExisting(t *testing.T) {
	root := testProject(t)
	parent := "wrk-99999999"
	for _, opts := range []CreateOptions{{Title: " "}, {Title: "Valid", Parent: &parent}, {Title: "Valid", Labels: []string{""}, LabelsSet: true}} {
		r := Create(root, opts)
		if r.Created || len(r.Diagnostics) == 0 {
			t.Fatal(r)
		}
		s := project.Load(root)
		if len(s.Diagnostics) > 0 || len(s.Tickets) != 1 || string(s.Tickets[0].Source) != testSource {
			t.Fatal("invalid creation changed project")
		}
	}
}
