package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"wrk/internal/diagnostic"
	"wrk/internal/store"
	"wrk/internal/ticket"
)

func TestScopeArguments(t *testing.T) {
	good := [][]string{
		{"list", "--ready", "--under=id", "--label=a", "--label=a", "--label=b"},
		{"update", "id", "--add-label=a", "--remove-label=b", "--add-label=a", "--remove-label=b", "--recursive"},
		{"update", "id", "--status=blocked", "--title=T", "--add-label=--dash"},
	}
	for _, args := range good {
		if _, err := Parse(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	bad := [][]string{
		{"update", "id", "--recursive"},
		{"update", "id", "--recursive", "--title=T", "--add-label=a"},
		{"update", "id", "--recursive", "--status=todo", "--remove-label=a"},
		{"update", "id", "--recursive", "--priority=high", "--add-label=a"},
		{"update", "id", "--add-label=a", "--remove-label=a"},
		{"update", "id", "--add-label="}, {"update", "id", "--remove-label="},
		{"update", "id", "--add-label=\xff"}, {"list", "--label=\xff"},
		{"update", "id", "--add-label"}, {"update", "id", "--label=a"},
		{"update", "id", "--add-label=a", "--no-labels"},
		{"update", "id", "--add-label=a", "--recursive", "--recursive"},
		{"list", "--label="}, {"list", "--under=a", "--under=b"}, {"list", "--under"},
		{"list", "--all", "--ready", "--label=a"}, {"list", "--recursive"},
	}
	for _, args := range bad {
		var out, errout bytes.Buffer
		if code := Run(append(args, "--json"), t.TempDir(), nil, &out, &errout); code != 2 {
			t.Fatalf("%v: %d %s", args, code, out.String())
		}
		var e Envelope
		if err := json.Unmarshal(out.Bytes(), &e); err != nil || e.Result != nil || e.ProjectRoot != nil || errout.Len() != 0 || e.Errors[0].Code != "USAGE" {
			t.Fatalf("%s %v", out.String(), err)
		}
	}
}

func TestRecursiveErrorOutput(t *testing.T) {
	root := t.TempDir()
	store.Init(root)
	parent := store.Create(root, store.CreateOptions{Title: "Parent"})
	child := store.Create(root, store.CreateOptions{Title: "Child", Parent: &parent.Ticket.ID})
	m := store.UpdateWithOptions(root, parent.Ticket.ID, store.UpdateOptions{Changes: ticket.Changes{AddLabels: []string{"burn"}}, Recursive: true})
	if len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	m.Diagnostics = append(m.Diagnostics, diagnostic.New("IO", "injected later rename failure", child.Ticket.Path))
	m.Updates[1].Publication = "pending"
	for _, jsonMode := range []bool{false, true} {
		var out, errout bytes.Buffer
		r := Request{Command: "update", JSON: jsonMode, Recursive: true}
		e := Envelope{SchemaVersion: 1, Command: "update", ProjectRoot: &root}
		if code := renderMutation(&out, &errout, r, e, m); code != 1 {
			t.Fatal(code)
		}
		if jsonMode {
			var got struct {
				OK     bool
				Result struct {
					Publication string
					Changed     bool
					Updates     []struct {
						Publication string
						Changed     bool
					}
				}
			}
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.OK || !got.Result.Changed || got.Result.Publication != "committed" || len(got.Result.Updates) != 2 || got.Result.Updates[1].Changed || got.Result.Updates[1].Publication != "pending" || errout.Len() != 0 {
				t.Fatal(out.String())
			}
		} else if !strings.Contains(out.String(), "committed") || !strings.Contains(out.String(), "pending") || !strings.Contains(out.String(), "inspect") || !strings.Contains(errout.String(), "IO") {
			t.Fatalf("%s %s", out.String(), errout.String())
		}
	}
	m.Committed = false
	var out, errout bytes.Buffer
	renderMutation(&out, &errout, Request{JSON: true}, Envelope{}, m)
	var e Envelope
	if err := json.Unmarshal(out.Bytes(), &e); err != nil || e.Result != nil {
		t.Fatal(out.String(), err)
	}
}
