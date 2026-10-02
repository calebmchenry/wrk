package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestScopedBurnWorkflow(t *testing.T) {
	root := t.TempDir()
	run(t, root, "", 0, "init", "--json")
	// Synthetic graph: dependencies cross both label and parent scopes.
	fixtures := []struct{ id, status, parent, deps, labels string }{
		{"00000001", "todo", "", "", "burn, backend"},
		{"00000002", "todo", "00000001", "00000007", "burn, backend, backend"},
		{"00000003", "todo", "00000002", "", "burn"},
		{"00000004", "blocked", "00000002", "00000007", "burn, backend"},
		{"00000005", "done", "00000001", "", "burn, backend"},
		{"00000006", "in-progress", "", "", "burn, backend"},
		{"00000007", "canceled", "", "", "external"},
	}
	for _, f := range fixtures {
		front := fmt.Sprintf("---\nid: wrk-%s\ntitle: Ticket %s\nstatus: %s\nlabels: [%s]\n", f.id, f.id, f.status, f.labels)
		if f.parent != "" {
			front += "parent: wrk-" + f.parent + "\n"
		}
		if f.deps != "" {
			front += "depends_on: [wrk-" + f.deps + "]\n"
		}
		put(t, root, ".wrk/wrk-"+f.id+".md", front+"---\nResume here: exact 🦊\r\nno newline")
	}
	check := func(want []string, args ...string) {
		t.Helper()
		e := run(t, root, "", 0, append(append([]string{"list"}, args...), "--json")...)
		tickets := result(t, e)["tickets"].([]any)
		got := []string{}
		for _, raw := range tickets {
			got = append(got, strings.TrimPrefix(raw.(map[string]any)["id"].(string), "wrk-"))
		}
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%v: got %v want %v", args, got, want)
		}
		cmd := exec.Command(binary, append([]string{"list"}, args...)...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(string(out), err)
		}
		lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
		if len(lines) != len(want)+1 {
			t.Fatalf("human scope: %s", out)
		}
		for i, id := range want {
			if !strings.HasPrefix(lines[i+1], "wrk-"+id+"\t") {
				t.Fatalf("human sort: %s", out)
			}
		}
	}
	run(t, root, "", 0, "validate", "--json")
	before := inventory(t, filepath.Join(root, ".wrk"))
	check([]string{"00000001", "00000002", "00000003", "00000004", "00000006"})
	check([]string{"00000001", "00000003"}, "--ready", "--label=burn")
	check([]string{"00000003"}, "--ready", "--under=wrk-00000001")
	check([]string{"00000002", "00000003", "00000004", "00000005"}, "--all", "--under=wrk-00000001", "--label=burn")
	check([]string{"00000002", "00000004"}, "--under=wrk-00000001", "--label=burn", "--label=backend", "--label=backend")
	check([]string{}, "--under=wrk-00000001", "--label=burn", "--label=external")
	check([]string{}, "--under=wrk-00000003")
	check([]string{}, "--label=Burn")
	for _, id := range []string{"wrk-deadbeef", "bad", ""} {
		e := run(t, root, "", 1, "list", "--ready", "--under="+id, "--label=absent", "--json")
		if e.Errors[0].Code != "NOT_FOUND" {
			t.Fatal(e.Errors)
		}
	}
	if !reflect.DeepEqual(before, inventory(t, filepath.Join(root, ".wrk"))) {
		t.Fatal("scope reads mutated project")
	}
	// A blocked prerequisite does not satisfy readiness; completion unblocks the
	// dependency but never clears the dependent's explicit blocked status.
	run(t, root, "", 0, "update", "wrk-00000007", "--status=blocked", "--json")
	check([]string{"00000003"}, "--ready", "--under=wrk-00000001")
	e := run(t, root, "", 0, "show", "wrk-00000002", "--json")
	blockers := result(t, e)["ticket"].(map[string]any)["blockers"].([]any)
	if len(blockers) != 1 || blockers[0].(map[string]any)["status"] != "blocked" {
		t.Fatal(blockers)
	}
	run(t, root, "", 0, "update", "wrk-00000007", "--status=done", "--json")
	check([]string{"00000002", "00000003"}, "--ready", "--under=wrk-00000001")
	e = run(t, root, "", 0, "show", "wrk-00000004", "--json")
	if result(t, e)["ticket"].(map[string]any)["status"] != "blocked" {
		t.Fatal("manual block cleared")
	}
	run(t, root, "", 0, "update", "wrk-00000004", "--status=todo", "--json")
	check([]string{"00000002", "00000003", "00000004"}, "--ready", "--under=wrk-00000001")
	// Completing the root does not change descendants or readiness.
	run(t, root, "", 0, "update", "wrk-00000001", "--status=done", "--json")
	check([]string{"00000002", "00000003", "00000004"}, "--ready", "--under=wrk-00000001")
	e = run(t, root, "", 0, "update", "wrk-00000004", "--status=in-progress", "--title=Resume work", "--add-label=resume", "--remove-label=backend", "--json")
	summary := result(t, e)["ticket"].(map[string]any)
	if !reflect.DeepEqual(summary["labels"], []any{"burn", "resume"}) || summary["status"] != "in-progress" || summary["title"] != "Resume work" {
		t.Fatal(summary)
	}
	// Recursive output includes root and completed descendants; unrelated work is untouched.
	e = run(t, root, "", 0, "update", "wrk-00000001", "--add-label=batch", "--recursive", "--json")
	updates := result(t, e)["updates"].([]any)
	if len(updates) != 5 || result(t, e)["changed"] != true {
		t.Fatal(string(e.Result))
	}
	for i, raw := range updates {
		entry := raw.(map[string]any)
		if entry["publication"] != "committed" || entry["ticket"].(map[string]any)["id"] != fmt.Sprintf("wrk-%08d", i+1) {
			t.Fatal(entry)
		}
	}
	check([]string{"00000001", "00000002", "00000003", "00000004", "00000005"}, "--all", "--label=batch")
	e = run(t, root, "", 0, "update", "wrk-00000001", "--add-label=batch", "--recursive", "--json")
	if result(t, e)["changed"] != false {
		t.Fatal("no-op changed")
	}
	for _, raw := range result(t, e)["updates"].([]any) {
		if raw.(map[string]any)["publication"] != "unchanged" {
			t.Fatal(raw)
		}
	}
	cmd := exec.Command(binary, "update", "wrk-00000001", "--remove-label=batch", "--recursive")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil || strings.Count(string(out), "committed") != 5 {
		t.Fatal(string(out), err)
	}
	check([]string{}, "--all", "--label=batch")
	raw, err := os.ReadFile(filepath.Join(root, ".wrk/wrk-00000004.md"))
	if err != nil || !strings.HasSuffix(string(raw), "Resume here: exact 🦊\r\nno newline") {
		t.Fatal("body not preserved", err)
	}
	run(t, root, "", 0, "validate", "--json")
}
