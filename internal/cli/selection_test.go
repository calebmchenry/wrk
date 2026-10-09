package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestSelectorArguments(t *testing.T) {
	for _, command := range [][]string{
		{"new", "Title"}, {"list"}, {"show", "id"},
		{"update", "id", "--status=done"}, {"validate"},
	} {
		for _, flag := range []string{"--project", "--config"} {
			for _, args := range [][]string{
				append([]string{flag, "a path"}, command...),
				append(append([]string{}, command...), flag+"=a path"),
			} {
				r, err := Parse(args)
				if err != nil {
					t.Fatal(args, err)
				}
				value := r.Selection.Project
				if flag == "--config" {
					value = r.Selection.Config
				}
				if value == nil || *value != "a path" || r.Command != command[0] {
					t.Fatalf("%v: %+v", args, r)
				}
			}
		}
	}
	bad := [][]string{
		{"--project=a", "list", "--config=b"}, {"--config=b", "list", "--project=a"},
		{"--project=a", "list", "--project=b"}, {"--config=a", "list", "--config=b"},
		{"list", "--project"}, {"--config"}, {"list", "--project", "--json"},
		{"list", "--config", "--json"}, {"list", "--project="}, {"--config=", "list"},
		{"--project=a"}, {"--config=a", "--help"},
		{"update", "id", "--project=a"}, {"list", "--", "--project=a"},
	}
	for _, command := range [][]string{{"init"}, {"help", "list"}, {"version"}, {"--version"}, {"upgrade", "--check"}} {
		for _, flag := range []string{"--project=a", "--config=a"} {
			bad = append(bad, append([]string{flag}, command...))
			bad = append(bad, append(append([]string{}, command...), flag))
		}
	}
	for _, args := range bad {
		if _, err := Parse(args); err == nil {
			t.Fatal("accepted", args)
		}
	}
	r, err := Parse([]string{"--project=a", "new", "--", "--config=b"})
	if err != nil || len(r.Args) != 1 || r.Args[0] != "--config=b" || r.Selection.Config != nil {
		t.Fatal("end of options", r, err)
	}
}

func TestSelectedCommandHelpDoesNotResolve(t *testing.T) {
	for _, args := range [][]string{
		{"--project=missing", "new", "--help", "--json"},
		{"list", "--config=not-a-config", "--help", "--json"},
	} {
		var out, errout bytes.Buffer
		code := Run(args, filepath.Join(t.TempDir(), "missing-cwd"), nil, &out, &errout)
		var e Envelope
		if err := json.Unmarshal(out.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		if code != 0 || !e.OK || e.ProjectRoot != nil || errout.Len() != 0 {
			t.Fatalf("help resolved project: %d %s %s", code, out.String(), errout.String())
		}
	}
}
