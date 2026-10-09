package integration

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExplicitProjectWorkflow(t *testing.T) {
	for _, flag := range []string{"--project", "--config"} {
		t.Run(flag, func(t *testing.T) {
			launch := t.TempDir()
			run(t, launch, "", 0, "init", "--json")
			local := newID(t, run(t, launch, "", 0, "new", "Local ticket", "--json"))
			selected := filepath.Join(t.TempDir(), "selected project")
			if err := os.Mkdir(selected, 0755); err != nil {
				t.Fatal(err)
			}
			// Init keeps its positional target, relative to the launch directory.
			relative, err := filepath.Rel(launch, selected)
			if err != nil {
				t.Fatal(err)
			}
			run(t, launch, "", 0, "init", relative, "--json")
			path := relative
			if flag == "--config" {
				path = filepath.Join(path, ".wrk/config.yaml")
			}
			beforeLocal := inventory(t, filepath.Join(launch, ".wrk"))
			put(t, launch, "body.md", "Body from the invocation directory 🦊\r\n")
			put(t, selected, "body.md", "Wrong body from the selected project")
			created := run(t, launch, "", 0, flag, path, "new", "Selected ticket", "--body-file=body.md", "--json")
			id := newID(t, created)
			if created.Root == nil || *created.Root != selected {
				t.Fatal("wrong selected root", created.Root)
			}
			initial := run(t, launch, "", 0, "show", id, flag, path, "--json")
			if !strings.HasSuffix(result(t, initial)["source"].(string), "Body from the invocation directory 🦊\r\n") {
				t.Fatal("creation did not use cwd-relative body input", result(t, initial))
			}
			put(t, launch, "body.md", "Replacement from cwd without final newline")
			run(t, launch, "", 0, "update", id, "--body-file=body.md", "--status=in-progress", flag+"="+path, "--json")
			e := run(t, launch, "", 0, "show", flag, path, id, "--json")
			if !strings.HasSuffix(result(t, e)["source"].(string), "Replacement from cwd without final newline") || result(t, e)["ticket"].(map[string]any)["status"] != "in-progress" {
				t.Fatal("selected update or cwd-relative body failed", result(t, e))
			}
			nested := filepath.Join(launch, "nested dir")
			if err := os.Mkdir(nested, 0755); err != nil {
				t.Fatal(err)
			}
			beforeSelected := inventory(t, filepath.Join(selected, ".wrk"))
			for _, cwd := range []string{nested, t.TempDir()} {
				absolute := selected
				if flag == "--config" {
					absolute = filepath.Join(absolute, ".wrk/config.yaml")
				}
				e = run(t, cwd, "", 0, "list", flag, absolute, "--json")
				tickets := result(t, e)["tickets"].([]any)
				if e.Root == nil || *e.Root != selected || len(tickets) != 1 || tickets[0].(map[string]any)["id"] != id {
					t.Fatal("wrong project listed", result(t, e))
				}
				run(t, cwd, "", 0, flag+"="+absolute, "validate", "--json")
			}
			if !reflect.DeepEqual(beforeSelected, inventory(t, filepath.Join(selected, ".wrk"))) {
				t.Fatal("selected reads wrote files")
			}
			// IDs and relationships never leak across projects.
			run(t, launch, "", 1, "show", local, flag, path, "--json")
			run(t, launch, "", 1, "update", local, "--status=done", flag, path, "--json")
			run(t, launch, "", 1, "new", "Bad parent", "--parent", local, flag, path, "--json")
			if !reflect.DeepEqual(beforeSelected, inventory(t, filepath.Join(selected, ".wrk"))) {
				t.Fatal("failed selected mutation changed project")
			}
			if !reflect.DeepEqual(beforeLocal, inventory(t, filepath.Join(launch, ".wrk"))) {
				t.Fatal("explicit selection mutated the launch project")
			}
			// The unselected command still discovers the nearest project.
			e = run(t, nested, "", 0, "list", "--json")
			if e.Root == nil || *e.Root != launch || result(t, e)["tickets"].([]any)[0].(map[string]any)["id"] != local {
				t.Fatal("default discovery changed", e)
			}
		})
	}
}

func TestExplicitSelectionNeverFallsBack(t *testing.T) {
	for _, kind := range []string{"missing-root", "missing-boundary", "missing-config", "invalid-config"} {
		t.Run(kind, func(t *testing.T) {
			outer := t.TempDir()
			run(t, outer, "", 0, "init", "--json")
			id := newID(t, run(t, outer, "", 0, "new", "Outer ticket", "--json"))
			selected := filepath.Join(outer, "nested")
			if kind != "missing-root" {
				if err := os.Mkdir(selected, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "missing-config" || kind == "invalid-config" {
				if err := os.Mkdir(filepath.Join(selected, ".wrk"), 0755); err != nil {
					t.Fatal(err)
				}
				if kind == "invalid-config" {
					put(t, selected, ".wrk/config.yaml", "version: 999\nprefix: wrk\n")
				}
			}
			before := inventory(t, filepath.Join(outer, ".wrk"))
			for _, selector := range [][]string{{"--project", selected}, {"--config", filepath.Join(selected, ".wrk/config.yaml")}} {
				for _, command := range [][]string{{"list"}, {"validate"}, {"new", "Must fail"}, {"update", id, "--status=done"}} {
					args := append(append(append([]string{}, selector...), command...), "--json")
					e := run(t, outer, "", 1, args...)
					if e.Root == nil || *e.Root != selected {
						t.Fatal("failure did not identify selected project", e)
					}
				}
			}
			if !reflect.DeepEqual(before, inventory(t, filepath.Join(outer, ".wrk"))) {
				t.Fatal("invalid selector mutated outer project")
			}
			if kind == "missing-root" || kind == "missing-boundary" || kind == "missing-config" {
				if _, err := os.Stat(filepath.Join(selected, ".wrk/config.yaml")); !os.IsNotExist(err) {
					t.Fatal("selection initialized project", err)
				}
			}
		})
	}
}

func TestSelectorUsageErrorsDoNotAccessProjects(t *testing.T) {
	root := t.TempDir()
	run(t, root, "", 0, "init", "--json")
	before := inventory(t, root)
	for _, args := range [][]string{
		{"--project=.", "list", "--config=.wrk/config.yaml"},
		{"--config=.wrk/config.yaml", "list", "--project=."},
		{"--project=.", "init"}, {"version", "--config=missing"},
		{"--project=missing", "upgrade", "--check"}, {"help", "--project=."},
		{"list", "--project="}, {"list", "--config="},
		{"--project=.", "list", "--project=."},
	} {
		e := run(t, root, "", 2, append(args, "--json")...)
		if e.Root != nil || e.Errors[0].Code != "USAGE" {
			t.Fatal(e)
		}
	}
	if !reflect.DeepEqual(before, inventory(t, root)) {
		t.Fatal("usage error accessed project")
	}
}
