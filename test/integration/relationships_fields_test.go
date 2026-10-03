package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"wrk/internal/schema"
	"wrk/internal/ticket"
)

func TestRelationshipAndCustomFieldWorkflow(t *testing.T) {
	root := t.TempDir()
	run(t, root, "", 0, "init", "--json")
	put(t, root, ".wrk/config.yaml", "version: 1\nprefix: wrk\nfields: {estimate: {type: number}, review: {type: boolean}, customer: {type: string}, area: {type: enum, options: [cli, docs]}}\n")
	parent := newID(t, run(t, root, "", 0, "new", "Parent", "--json"))
	prerequisite := newID(t, run(t, root, "", 0, "new", "Prerequisite", "--json"))
	body := "\r\nExact 🦊\r\n---\nno newline"
	child := newID(t, run(t, root, body, 0, "new", "Child", "--body-file=-", "--parent", parent, "--depends-on", prerequisite,
		"--field=estimate=!!int 123456789012345678901234567890", "--field=review=true", "--field=customer='true'", "--field=area=cli",
		"--field=opaque=&loop !tag [*loop]", "--json"))
	summary := func(e envelope) map[string]any { return result(t, e)["ticket"].(map[string]any) }
	show := func(id string) envelope { return run(t, root, "", 0, "show", id, "--json") }
	e := show(child)
	s := summary(e)
	if s["parent"] != parent || !reflect.DeepEqual(s["depends_on"], []any{prerequisite}) || len(s["blockers"].([]any)) != 1 {
		t.Fatal(string(e.Result))
	}
	if len(result(t, show(parent))["children"].([]any)) != 1 {
		t.Fatal("missing derived child")
	}
	if len(result(t, run(t, root, "", 0, "list", "--ready", "--under", parent, "--json"))["tickets"].([]any)) != 0 {
		t.Fatal("blocked child ready")
	}
	source := result(t, e)["source"].(string)
	beforeTicket, ds := ticket.Parse([]byte(source), "fixture")
	if len(ds) > 0 || !strings.HasSuffix(source, body) {
		t.Fatal(source, ds)
	}
	path := filepath.Join(root, ".wrk", child+".md")
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	// Reparent and alter dependencies/custom values in one validated publication.
	e = run(t, root, "", 0, "update", child, "--parent", prerequisite, "--remove-dependency", prerequisite, "--add-dependency", parent,
		"--field=estimate=1.23456789012345678901234567890", "--remove-field=review", "--field=area=docs", "--field=new=null", "--status=done", "--json")
	if result(t, e)["changed"] != true || summary(e)["parent"] != prerequisite || summary(e)["status"] != "done" {
		t.Fatal(string(e.Result))
	}
	e = show(child)
	afterTicket, ds := ticket.Parse([]byte(result(t, e)["source"].(string)), "fixture")
	af := schema.Map(schema.Map(afterTicket.Node)["fields"])
	bf := schema.Map(schema.Map(beforeTicket.Node)["fields"])
	info, _ := os.Stat(path)
	if len(ds) > 0 || string(afterTicket.Body) != body || info.Mode().Perm() != 0640 || af["review"] != nil || af["new"].Tag != "!!null" || !ticket.Equal(bf["opaque"], af["opaque"]) || !ticket.Equal(bf["customer"], af["customer"]) {
		t.Fatal("lost custom values/body/mode", ds)
	}
	if len(result(t, show(parent))["children"].([]any)) != 0 || len(result(t, show(prerequisite))["children"].([]any)) != 1 || summary(show(parent))["status"] != "todo" {
		t.Fatal("derived relationships/cascade")
	}
	// Retries preserve exact bytes and inode, and human output reports unchanged.
	before := inventory(t, filepath.Join(root, ".wrk"))
	cmd := exec.Command(binary, "update", child, "--parent", prerequisite, "--add-dependency", parent, "--add-dependency", parent,
		"--remove-dependency=wrk-deadbeef", "--field=area=docs", "--remove-field=review")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(out), "(unchanged)") {
		t.Fatal(string(out), err)
	}
	noOpInfo, _ := os.Stat(path)
	if !os.SameFile(info, noOpInfo) || !reflect.DeepEqual(before, inventory(t, filepath.Join(root, ".wrk"))) {
		t.Fatal("no-op wrote files")
	}
	// Both cycles and invalid typed values reject the whole combined update.
	for _, args := range [][]string{
		{"update", prerequisite, "--parent", child},
		{"update", parent, "--add-dependency", child},
		{"update", child, "--parent", child},
		{"update", child, "--add-dependency", child},
		{"update", child, "--parent=wrk-deadbeef"},
		{"update", child, "--add-dependency=wrk-deadbeef"},
		{"update", child, "--title=Must not publish", "--no-parent", "--field=review='true'"},
		{"update", child, "--title=Must not publish", "--field=estimate='2'"},
		{"new", "Invalid", "--depends-on", parent, "--depends-on", parent},
		{"new", "Invalid", "--depends-on=wrk-deadbeef"},
		{"new", "Invalid", "--field=customer=false"},
		{"new", "Invalid", "--field=area=unknown"},
	} {
		run(t, root, "", 1, append(args, "--json")...)
		if !reflect.DeepEqual(before, inventory(t, filepath.Join(root, ".wrk"))) {
			t.Fatal("failed mutation changed inputs", args)
		}
	}
	// Explicit reopening is permitted despite blockers. Finishing a dependency
	// changes readiness without modifying the dependent ticket's source.
	run(t, root, "", 0, "update", child, "--status=todo", "--json")
	childSource := result(t, show(child))["source"]
	run(t, root, "", 0, "update", parent, "--status=done", "--json")
	if result(t, show(child))["source"] != childSource {
		t.Fatal("dependency completion rewrote child")
	}
	ready := result(t, run(t, root, "", 0, "list", "--ready", "--under", prerequisite, "--json"))["tickets"].([]any)
	if len(ready) != 1 || ready[0].(map[string]any)["id"] != child {
		t.Fatal("readiness not refreshed", ready)
	}
	run(t, root, "", 0, "update", child, "--no-parent", "--remove-dependency", parent,
		"--remove-field=estimate", "--remove-field=customer", "--remove-field=area", "--remove-field=opaque", "--remove-field=new", "--json")
	e = show(child)
	if summary(e)["parent"] != nil || len(summary(e)["depends_on"].([]any)) != 0 || !strings.Contains(result(t, e)["source"].(string), "fields: {}") {
		t.Fatal(string(e.Result))
	}
	run(t, root, "", 0, "validate", "--json")
}
