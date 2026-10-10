package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var binary string

func TestMain(m *testing.M) {
	if os.Getenv("WRK_RUN_TEST_HELPER") == "1" {
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "wrk-integration-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binary = filepath.Join(dir, "wrk")
	cmd := exec.Command("go", "build", "-o", binary, "../../cmd/wrk")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		fmt.Fprintln(os.Stderr, string(out), err)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type envelope struct {
	SchemaVersion int                                    `json:"schema_version"`
	Command       string                                 `json:"command"`
	OK            bool                                   `json:"ok"`
	Root          *string                                `json:"project_root"`
	Result        json.RawMessage                        `json:"result"`
	Errors        []struct{ Code, Message, Path string } `json:"errors"`
}

func run(t *testing.T, cwd, input string, want int, args ...string) envelope {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if e, ok := err.(*exec.ExitError); ok {
		code = e.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	if code != want {
		t.Fatalf("%v: exit %d want %d\nstdout %s\nstderr %s", args, code, want, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("JSON stderr %s", stderr.String())
	}
	var e envelope
	dec := json.NewDecoder(&stdout)
	if err := dec.Decode(&e); err != nil {
		t.Fatal(err)
	}
	if dec.Decode(new(any)) != io.EOF {
		t.Fatal("extra JSON output")
	}
	if e.SchemaVersion != 1 || e.Errors == nil || e.OK != (want == 0) {
		t.Fatalf("invalid envelope: %+v", e)
	}
	if want != 0 && (len(e.Errors) == 0 || string(e.Result) != "null") {
		t.Fatalf("invalid error: %+v", e)
	}
	return e
}
func result(t *testing.T, e envelope) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(e.Result, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
func newID(t *testing.T, e envelope) string {
	t.Helper()
	return result(t, e)["ticket"].(map[string]any)["id"].(string)
}
func inventory(t *testing.T, root string) map[string]string {
	t.Helper()
	m := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			m[rel] = "DIR"
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			m[rel] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func put(t *testing.T, root, path, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteWorkflow(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"--json"}, {"--json", "--help"}, {"help", "--json"}, {"help", "update", "--json"}, {"new", "--help", "--json"}} {
		e := run(t, root, "", 0, args...)
		if e.Root != nil {
			t.Fatal("help discovered project")
		}
	}
	run(t, root, "", 0, "init", "--json")
	parent := newID(t, run(t, root, "", 0, "--json", "new", "Parent"))
	body := "\r\n## Body 🦊\r\n---\nno terminal newline"
	child := newID(t, run(t, root, body, 0, "new", "Child", "--body-file=-", "--parent", parent, "--priority=high", "--label=cli", "--label=storage", "--json"))
	path := ".wrk/" + child + ".md"
	raw, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	e := run(t, root, "", 0, "show", child, "--json")
	if result(t, e)["source"] != string(raw) || !bytes.HasSuffix(raw, []byte(body)) {
		t.Fatal("source/body lost bytes")
	}
	nested := filepath.Join(root, "deep", "nested")
	os.MkdirAll(nested, 0755)
	e = run(t, nested, "", 0, "--json", "list", "--ready")
	if len(result(t, e)["tickets"].([]any)) != 2 || e.Root == nil || *e.Root != root {
		t.Fatal("nested/ready behavior")
	}
	before := inventory(t, filepath.Join(root, ".wrk"))
	for _, args := range [][]string{{"list", "--json"}, {"list", "--all", "--json"}, {"show", parent, "--json"}, {"validate", "--json"}} {
		run(t, nested, "", 0, args...)
	}
	if !reflect.DeepEqual(before, inventory(t, filepath.Join(root, ".wrk"))) {
		t.Fatal("reads changed project")
	}
	run(t, nested, "", 0, "update", child, "--status=in-progress", "--json")
	updated, _ := os.ReadFile(filepath.Join(root, path))
	directBody := body + "\nDirect body edit"
	updated = append(bytes.TrimSuffix(updated, []byte(body)), []byte(directBody)...)
	put(t, root, path, string(updated))
	e = run(t, nested, "", 0, "update", "--title= Child renamed ", child, "--status=done", "--json")
	if result(t, e)["changed"] != true {
		t.Fatal("combined update")
	}
	e = run(t, root, "", 0, "show", child, "--json")
	summary := result(t, e)["ticket"].(map[string]any)
	if summary["title"] != " Child renamed " || summary["status"] != "done" || summary["path"] != path || !strings.HasSuffix(result(t, e)["source"].(string), directBody) {
		t.Fatal("updated result")
	}
	e = run(t, root, "", 0, "update", child, "--status=done", "--json")
	if result(t, e)["changed"] != false {
		t.Fatal("no-op")
	}
	e = run(t, root, "", 0, "list", "--json")
	if len(result(t, e)["tickets"].([]any)) != 1 {
		t.Fatal("active list")
	}
	e = run(t, root, "", 0, "list", "--all", "--json")
	if len(result(t, e)["tickets"].([]any)) != 2 {
		t.Fatal("all list")
	}
	e = run(t, root, "", 0, "show", parent, "--json")
	if result(t, e)["ticket"].(map[string]any)["status"] != "todo" || len(result(t, e)["children"].([]any)) != 1 {
		t.Fatal("cascade or children")
	}
	run(t, root, "", 0, "validate", "--json")
	// A second explicit init may create a nested project, without using discovery.
	run(t, root, "", 0, "init", nested, "--json")
	e = run(t, nested, "", 0, "list", "--json")
	if len(result(t, e)["tickets"].([]any)) != 0 {
		t.Fatal("nested project fell through")
	}
}

func TestCommandErrorsAndStrictProject(t *testing.T) {
	root := t.TempDir()
	run(t, root, "", 1, "list", "--json")
	run(t, root, "", 0, "init", "--json")
	id := newID(t, run(t, root, "", 0, "new", "Existing", "--json"))
	for _, args := range [][]string{
		{"init", "--json"}, {"new", "Bad", "--parent=wrk-deadbeef", "--json"}, {"show", "wrk-deadbeef", "--json"}, {"update", "wrk-deadbeef", "--status=done", "--json"}, {"update", id, "--status=invalid", "--json"}, {"new", "Bad", "--priority=invalid", "--json"}, {"new", "Bad", "--body-file=missing-file", "--json"},
	} {
		run(t, root, "", 1, args...)
	}
	for _, args := range [][]string{
		{"new", "--json"}, {"list", "--all", "--ready", "--json"}, {"new", "T", "--label=a", "--no-labels", "--json"}, {"update", id, "--json"}, {"update", id, "--status=todo", "--status=done", "--json"}, {"list", "extra", "--json"}, {"show", id, "--unknown", "--json"}, {"init", "a", "b", "--json"},
	} {
		run(t, root, "", 2, args...)
	}
	put(t, root, ".wrk/wrk-00000001.md", "---\nid: wrk-00000001\ntitle: bad\nstatus: nonsense\n---\n")
	before := inventory(t, filepath.Join(root, ".wrk"))
	for _, args := range [][]string{{"validate", "--json"}, {"list", "--json"}, {"show", id, "--json"}, {"new", "No partial result", "--json"}, {"update", id, "--status=done", "--json"}} {
		e := run(t, root, "", 1, args...)
		if len(e.Errors) < 2 {
			t.Fatal("missing diagnostics")
		}
	}
	if !reflect.DeepEqual(before, inventory(t, filepath.Join(root, ".wrk"))) {
		t.Fatal("invalid project mutated")
	}
}

func TestConfiguredDefaultsCustomFieldsAndFileBodies(t *testing.T) {
	root := t.TempDir()
	run(t, root, "", 0, "init", "--json")
	put(t, root, ".wrk/config.yaml", "version: 1\nprefix: task\ndefaults: {priority: urgent, labels: [default]}\nfields: {estimate: {type: number}, review: {type: boolean}}\n")
	put(t, root, "body.txt", "\nUnicode 🦊\r\n---\nEOF")
	id := newID(t, run(t, root, "", 0, "new", "File body", "--body-file", "body.txt", "--json"))
	e := run(t, root, "", 0, "show", id, "--json")
	summary := result(t, e)["ticket"].(map[string]any)
	if !strings.HasPrefix(id, "task-") || summary["priority"] != "urgent" || !strings.HasSuffix(result(t, e)["source"].(string), "\nUnicode 🦊\r\n---\nEOF") {
		t.Fatal("defaults/body")
	}
	noLabels := newID(t, run(t, root, "", 0, "new", "No labels", "--no-labels", "--json"))
	e = run(t, root, "", 0, "show", noLabels, "--json")
	if len(result(t, e)["ticket"].(map[string]any)["labels"].([]any)) != 0 {
		t.Fatal("empty override")
	}
	// Synthetic fixture: direct metadata setup in a disposable project only.
	path := ".wrk/" + id + ".md"
	data, _ := os.ReadFile(filepath.Join(root, path))
	data = bytes.Replace(data, []byte("status: todo"), []byte("status: todo\nfields: {estimate: !!int 999999999999999999999999999999, review: true, opaque: !tag [1, 2]}"), 1)
	put(t, root, path, string(data))
	run(t, root, "", 0, "update", id, "--title=Changed", "--json")
	run(t, root, "", 0, "validate", "--json")
	data, _ = os.ReadFile(filepath.Join(root, path))
	put(t, root, path, strings.Replace(string(data), "review: true", "review: 'true'", 1))
	run(t, root, "", 1, "validate", "--json")
}

func TestHumanTerminalEscapingAndDashTitle(t *testing.T) {
	root := t.TempDir()
	run(t, root, "", 0, "init", "--json")
	id := newID(t, run(t, root, "", 0, "new", "--json", "--", "--dash"))
	cmd := exec.Command(binary, "show", id)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte("--dash")) || !bytes.Contains(out, []byte("Derived information")) {
		t.Fatalf("%s %v", out, err)
	}
	put(t, root, "body.txt", "escape \x1b[31mred\x1b[0m\rreturn")
	id = newID(t, run(t, root, "", 0, "new", "Terminal", "--body-file=body.txt", "--json"))
	cmd = exec.Command(binary, "show", id)
	cmd.Dir = root
	out, err = cmd.CombinedOutput()
	if err != nil || bytes.ContainsAny(out, "\x1b\r") || !bytes.Contains(out, []byte("\\u001b")) {
		t.Fatalf("unsafe human output %q %v", out, err)
	}
	cmd = exec.Command(binary, "list", "--wat")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "USAGE") {
		t.Fatal("human error routing")
	}
}
