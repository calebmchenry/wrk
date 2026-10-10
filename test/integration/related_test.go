package integration

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestRelatedCLIWorkflow(t *testing.T) {
	root := t.TempDir()
	run(t, root, "", 0, "init", "--json")
	a := newID(t, run(t, root, "", 0, "new", "A", "--json"))
	b := newID(t, run(t, root, "", 0, "new", "B", "--related", a, "--json"))
	c := newID(t, run(t, root, "", 0, "new", "C", "--related", a, "--related", b, "--json"))
	show := func(id string) map[string]any {
		return result(t, run(t, root, "", 0, "show", id, "--json"))["ticket"].(map[string]any)
	}
	if len(show(a)["related"].([]any)) != 2 {
		t.Fatal("missing reverse display")
	}
	before := show(a)
	e := run(t, root, "", 0, "update", a, "--add-related", b, "--add-related", b, "--json")
	if result(t, e)["changed"] != false || !reflect.DeepEqual(before, show(a)) {
		t.Fatal("reverse add changed source")
	}
	cmd := exec.Command(binary, "show", a)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "Related:") || !strings.Contains(string(out), b) || !strings.Contains(string(out), c) {
		t.Fatal(string(out), err)
	}
	for _, args := range [][]string{
		{"new", "Duplicate", "--related", a, "--related", a},
		{"new", "Missing", "--related=wrk-deadbeef"},
		{"update", a, "--add-related", a}, {"update", a, "--remove-related", a},
		{"update", a, "--add-related=wrk-deadbeef"},
	} {
		run(t, root, "", 1, append(args, "--json")...)
	}
	for _, args := range [][]string{
		{"update", a, "--add-related", b, "--remove-related", b},
		{"update", a, "--no-related", "--add-related", b},
		{"update", a, "--no-related", "--remove-related", b},
		{"update", a, "--no-related", "--recursive"},
		{"update", a, "--remove-related=bad"}, {"update", a, "--related", b},
		{"new", "Bad flags", "--add-related", b},
	} {
		run(t, root, "", 2, append(args, "--json")...)
	}
	if len(result(t, run(t, root, "", 0, "list", "--ready", "--json"))["tickets"].([]any)) != 3 {
		t.Fatal("links changed readiness")
	}
	e = run(t, root, "", 0, "update", a, "--remove-related", b, "--title=Changed", "--field=kept=true", "--json")
	if len(result(t, e)["updates"].([]any)) != 2 || show(a)["title"] != "Changed" || len(show(b)["related"].([]any)) != 1 {
		t.Fatal(string(e.Result))
	}
	e = run(t, root, "", 0, "update", a, "--remove-related", b, "--remove-related", b, "--json")
	if result(t, e)["changed"] != false {
		t.Fatal("remove not idempotent")
	}
	run(t, root, "", 0, "update", c, "--no-related", "--json")
	for _, id := range []string{a, b, c} {
		if len(show(id)["related"].([]any)) != 0 {
			t.Fatal("clear incomplete")
		}
	}
	run(t, root, "", 0, "validate", "--json")
}
