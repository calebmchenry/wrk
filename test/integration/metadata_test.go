package integration

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPriorityAndLabelUpdateWorkflow(t *testing.T) {
	root := t.TempDir()
	run(t, root, "", 0, "init", "--json")
	put(t, root, ".wrk/config.yaml", "version: 1\nprefix: wrk\ndefaults: {priority: high, labels: [default, default]}\nfields: {}\n")
	body := "\r\nExact 🦊\r\n---\nno newline"
	id := newID(t, run(t, root, body, 0, "new", "Before", "--body-file=-", "--json"))
	summary := func(e envelope) map[string]any { return result(t, e)["ticket"].(map[string]any) }
	e := run(t, root, "", 0, "update", id, "--title=After", "--status=blocked", "--priority=urgent", "--label=z", "--label=a", "--label=a", "--json")
	got := summary(e)
	if result(t, e)["changed"] != true || got["priority"] != "urgent" || got["title"] != "After" || got["status"] != "blocked" || !reflect.DeepEqual(got["labels"], []any{"a", "a", "z"}) {
		t.Fatal(string(e.Result))
	}
	e = run(t, root, "", 0, "show", id, "--json")
	source := result(t, e)["source"].(string)
	if !strings.HasSuffix(source, body) || !strings.Contains(source, "labels:\n  - z\n  - a\n  - a\n") {
		t.Fatal(source)
	}
	e = run(t, root, "", 0, "update", id, "--priority=urgent", "--label=z", "--label=a", "--label=a", "--json")
	if result(t, e)["changed"] != false {
		t.Fatal("no-op rewrote")
	}
	if gotSource := result(t, run(t, root, "", 0, "show", id, "--json"))["source"]; gotSource != source {
		t.Fatal("no-op changed source")
	}
	e = run(t, root, "", 0, "update", id, "--priority=low", "--add-label=keep", "--remove-label=a", "--json")
	if summary(e)["priority"] != "low" || !reflect.DeepEqual(summary(e)["labels"], []any{"keep", "z"}) {
		t.Fatal(string(e.Result))
	}
	e = run(t, root, "", 0, "update", id, "--no-labels", "--priority=normal", "--json")
	if result(t, e)["changed"] != true || summary(e)["priority"] != "normal" || !reflect.DeepEqual(summary(e)["labels"], []any{}) {
		t.Fatal(string(e.Result))
	}
	cmd := exec.Command(binary, "update", id, "--no-labels", "--priority=normal")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(out), id+" .wrk/"+id+".md (unchanged)") {
		t.Fatal(string(out), err)
	}
	cmd = exec.Command(binary, "update", id, "--priority=high", "--label=human")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != id+" .wrk/"+id+".md" {
		t.Fatal(string(out), err)
	}
	cmd = exec.Command(binary, "list")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(out), id+"\tblocked\thigh\tAfter") {
		t.Fatal(string(out), err)
	}
	// Replacing/clearing recursively follows the same label-only contract.
	child := newID(t, run(t, root, "", 0, "new", "Child", "--parent", id, "--label=other", "--json"))
	for _, flags := range [][]string{{"--label=batch", "--label=batch"}, {"--no-labels"}} {
		e = run(t, root, "", 0, append([]string{"update", id, "--recursive", "--json"}, flags...)...)
		updates := result(t, e)["updates"].([]any)
		if len(updates) != 2 || result(t, e)["changed"] != true {
			t.Fatal(string(e.Result))
		}
		for _, raw := range updates {
			if raw.(map[string]any)["publication"] != "committed" {
				t.Fatal(raw)
			}
		}
	}
	// A later creation-default change never alters old tickets or update meaning.
	put(t, root, ".wrk/config.yaml", "version: 1\nprefix: wrk\ndefaults: {priority: urgent, labels: [future]}\nfields: {}\n")
	e = run(t, root, "", 0, "update", id, "--status=todo", "--json")
	if summary(e)["priority"] != "high" || !reflect.DeepEqual(summary(e)["labels"], []any{}) {
		t.Fatal("defaults leaked into update", string(e.Result))
	}
	e = run(t, root, "", 0, "show", child, "--json")
	if summary(e)["priority"] != "high" || !reflect.DeepEqual(summary(e)["labels"], []any{}) {
		t.Fatal("defaults changed child", string(e.Result))
	}
	e = run(t, root, "", 0, "new", "Future", "--json")
	if summary(e)["priority"] != "urgent" || !reflect.DeepEqual(summary(e)["labels"], []any{"future"}) {
		t.Fatal(string(e.Result))
	}
	before := inventory(t, filepath.Join(root, ".wrk"))
	for _, priority := range []string{"", "URGENT", " high", "invalid"} {
		e = run(t, root, "", 1, "update", id, "--title=Must not publish", "--priority="+priority, "--no-labels", "--json")
		if e.Errors[0].Code != "INVALID_TICKET" {
			t.Fatal(e.Errors)
		}
	}
	for _, flags := range [][]string{
		{"--label=x", "--no-labels"}, {"--label=x", "--add-label=y"}, {"--no-labels", "--remove-label=x"},
		{"--priority=normal", "--recursive", "--label=x"}, {"--priority=high", "--priority=low"},
	} {
		run(t, root, "", 2, append([]string{"update", id, "--json"}, flags...)...)
	}
	if !reflect.DeepEqual(before, inventory(t, filepath.Join(root, ".wrk"))) {
		t.Fatal("invalid update changed inputs")
	}
	run(t, root, "", 0, "validate", "--json")
}
