package integration

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBodyUpdateWorkflow(t *testing.T) {
	root := t.TempDir()
	run(t, root, "", 0, "init", "--json")
	created := run(t, root, "Original", 0, "new", "Before", "--body-file=-", "--field", "opaque=!tag {exact: 123456789012345678901234567890}", "--json")
	id := newID(t, created)
	summary := func(e envelope) map[string]any { return result(t, e)["ticket"].(map[string]any) }
	initialRevision := summary(created)["revision"]
	if initialRevision == nil || initialRevision == "" {
		t.Fatal("missing revision", string(created.Result))
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	fileBody := "\r\nUpdated 🦊\r\n---\nno newline"
	put(t, nested, "body.txt", fileBody)
	e := run(t, nested, "", 0, "update", "--body-file=body.txt", id, "--title=After", "--status=in-progress", "--add-label=edited", "--json")
	if result(t, e)["changed"] != true || summary(e)["title"] != "After" || summary(e)["status"] != "in-progress" || summary(e)["revision"] == initialRevision {
		t.Fatal(string(e.Result))
	}
	revision := summary(e)["revision"]
	e = run(t, root, "", 0, "show", id, "--json")
	source := result(t, e)["source"].(string)
	if !strings.HasSuffix(source, fileBody) || !strings.Contains(source, "!tag") || !strings.Contains(source, "123456789012345678901234567890") || summary(e)["revision"] != revision {
		t.Fatal(source)
	}
	tickets := result(t, run(t, root, "", 0, "list", "--json"))["tickets"].([]any)
	if tickets[0].(map[string]any)["revision"] != revision {
		t.Fatal("list revision differs from show/update")
	}
	path := filepath.Join(root, ".wrk", id+".md")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	e = run(t, root, fileBody, 0, "update", id, "--body-file=-", "--json")
	after, err := os.Stat(path)
	if err != nil || result(t, e)["changed"] != false || summary(e)["revision"] != revision || !os.SameFile(before, after) {
		t.Fatal("no-op replaced file", err, string(e.Result))
	}
	// Empty input clears, and later metadata-only edits preserve the empty body.
	run(t, root, "", 0, "update", id, "--body-file=-", "--json")
	run(t, root, "", 0, "update", id, "--status=done", "--json")
	e = run(t, root, "", 0, "show", id, "--json")
	if !strings.HasSuffix(result(t, e)["source"].(string), "\n---\n") {
		t.Fatal("empty body not preserved", string(e.Result))
	}
	beforeFiles := inventory(t, filepath.Join(root, ".wrk"))
	for _, tc := range []struct {
		input, code string
		flags       []string
	}{
		{"\xff", "INVALID_BODY", []string{"--body-file=-", "--title=Must not publish"}},
		{"Valid draft", "INVALID_TICKET", []string{"--body-file=-", "--status=invalid"}},
		{"Valid draft", "MISSING_REFERENCE", []string{"--body-file=-", "--parent=wrk-deadbeef"}},
		{"", "IO", []string{"--body-file=missing.txt"}},
	} {
		e = run(t, root, tc.input, 1, append([]string{"update", id, "--json"}, tc.flags...)...)
		if e.Errors[0].Code != tc.code {
			t.Fatal(e.Errors)
		}
	}
	run(t, root, "Draft", 2, "update", id, "--body-file=-", "--label=x", "--recursive", "--json")
	if !reflect.DeepEqual(beforeFiles, inventory(t, filepath.Join(root, ".wrk"))) {
		t.Fatal("rejected update changed project")
	}
	run(t, root, "", 0, "validate", "--json")
}
