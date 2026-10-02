package project

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"wrk/internal/diagnostic"
)

func write(t *testing.T, root, path, content string) {
	t.Helper()
	name := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func seed(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, ConfigPath, DefaultConfig)
	return root
}
func source(id, extra string) string {
	return "---\nid: " + id + "\ntitle: Title\nstatus: todo\n" + extra + "---\nBody"
}
func hasCode(ds []diagnostic.Diagnostic, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			out[rel] = "DIR"
		} else {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[rel] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestOriginalCompatibilityReadOnly(t *testing.T) {
	root, err := filepath.Abs("../../testdata/compat")
	if err != nil {
		t.Fatal(err)
	}
	before := tree(t, root)
	s := Load(root)
	if len(s.Diagnostics) > 0 || len(s.Tickets) != 3 {
		t.Fatalf("%+v", s.Diagnostics)
	}
	if len(s.List(false, false)) != 2 || len(s.List(false, true)) != 2 || len(s.Children("wrk-3f8a21b7")) != 1 {
		t.Fatal("derived state")
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("read wrote files")
	}
}
func TestNearestBoundary(t *testing.T) {
	for _, kind := range []string{"missing-config", "bad-config", "file", "symlink", "valid"} {
		t.Run(kind, func(t *testing.T) {
			root := seed(t)
			nested := filepath.Join(root, "nested")
			os.Mkdir(nested, 0755)
			switch kind {
			case "file":
				write(t, nested, ".wrk", "bad")
			case "symlink":
				os.Symlink(filepath.Join(root, ".wrk"), filepath.Join(nested, ".wrk"))
			case "missing-config":
				os.Mkdir(filepath.Join(nested, ".wrk"), 0755)
			case "bad-config":
				write(t, nested, ConfigPath, "version: 2\nprefix: wrk\n")
			case "valid":
				write(t, nested, ConfigPath, DefaultConfig)
			}
			child := filepath.Join(nested, "sub")
			os.Mkdir(child, 0755)
			got, ds := Discover(child)
			if got != nested {
				t.Fatalf("fell through: %s %v", got, ds)
			}
			if kind == "file" || kind == "symlink" {
				if len(ds) == 0 {
					t.Fatal("accepted boundary")
				}
			} else if kind != "valid" && len(Load(got).Diagnostics) == 0 {
				t.Fatal("accepted config")
			}
		})
	}
}
func TestStrictConfiguration(t *testing.T) {
	bad := []string{
		"version: 1\nversion: 1\nprefix: wrk\n", "version: '1'\nprefix: wrk\n", "version: 2\nprefix: wrk\n", "version: 1\nprefix: Bad\n", "version: 1\nprefix: wrk\nunknown: yes\n", "version: 1\nprefix: wrk\n---\nmore: true\n",
		"version: 1\nprefix: wrk\ndefaults: null\n", "version: 1\nprefix: wrk\nfields: null\n", "version: 1\nprefix: wrk\ndefaults: {labels: null}\n", "version: 1\nprefix: wrk\ndefaults: {priority: urgent, priority: low}\n", "version: 1\nprefix: wrk\ndefaults: {status: todo}\n", "version: 1\nprefix: wrk\ndefaults: {priority: false}\n",
		"version: 1\nprefix: wrk\nfields: {a: {type: enum, options: []}}\n", "version: 1\nprefix: wrk\nfields: {a: {type: enum, options: [x, x]}}\n", "version: 1\nprefix: wrk\nfields: {a: {type: enum, options: [1]}}\n", "version: 1\nprefix: wrk\nfields: {a: {type: boolean, extra: true}}\n", "version: 1\nprefix: wrk\nfields: {a: {type: string, options: [x]}}\n",
	}
	for _, input := range bad {
		if _, ds := ParseConfig([]byte(input)); len(ds) == 0 {
			t.Errorf("accepted %q", input)
		}
	}
	for _, input := range []string{"version: 1\nprefix: new\n", DefaultConfig, "version: 1\nprefix: wrk\nfields: {a: {type: enum, options: ['yes', no]}}\n"} {
		if _, ds := ParseConfig([]byte(input)); len(ds) > 0 {
			t.Errorf("rejected %q: %+v", input, ds)
		}
	}
}
func TestTicketAndIdentityDiagnostics(t *testing.T) {
	root := seed(t)
	write(t, root, ".wrk/wrk-00000001.md", source("wrk-00000001", "priority: wrong\nlabels: [1]\nfields: {a: {same: 1, same: 2}}\n"))
	write(t, root, ".wrk/wrk-00000002.md", source("wrk-00000001", ""))
	s := Load(root)
	for _, code := range []string{"INVALID_TICKET", "ID_MISMATCH", "DUPLICATE_ID", "CHECK_UNAVAILABLE"} {
		if !hasCode(s.Diagnostics, code) {
			t.Fatalf("missing %s: %+v", code, s.Diagnostics)
		}
	}
	before := tree(t, root)
	Load(root)
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("validation wrote")
	}
}
func TestSymlinkCandidatesAndSupportingFiles(t *testing.T) {
	root := seed(t)
	write(t, root, ".wrk/notes.md", "not a ticket")
	write(t, root, ".wrk/.wrk-stage-residual", "unfinished")
	write(t, root, ".wrk/.lock", "lock")
	if ds := Load(root).Diagnostics; len(ds) > 0 {
		t.Fatal(ds)
	}
	os.Symlink(filepath.Join(root, ConfigPath), filepath.Join(root, ".wrk/wrk-00000001.md"))
	if !hasCode(Load(root).Diagnostics, "INVALID_TICKET") {
		t.Fatal("symlink accepted")
	}
	os.Remove(filepath.Join(root, ".wrk/wrk-00000001.md"))
	os.Mkdir(filepath.Join(root, ".wrk/wrk-00000001.md"), 0755)
	if !hasCode(Load(root).Diagnostics, "INVALID_TICKET") {
		t.Fatal("directory accepted")
	}
	os.Remove(filepath.Join(root, ConfigPath))
	os.Symlink(filepath.Join(root, ".wrk/notes.md"), filepath.Join(root, ConfigPath))
	if !hasCode(Load(root).Diagnostics, "INVALID_CONFIG") {
		t.Fatal("config symlink accepted")
	}
}
func TestStrictCustomTypes(t *testing.T) {
	for _, tc := range []struct {
		typ, value string
		valid      bool
	}{
		{"string", "yes", true}, {"string", "true", false}, {"number", "'42'", false}, {"number", "42", true}, {"number", "!!int 123456789012345678901234567890", true}, {"number", "1.5", true}, {"boolean", "yes", false}, {"boolean", "'true'", false}, {"boolean", "true", true}, {"enum", "'a'", true}, {"enum", "7", false},
	} {
		t.Run(tc.typ+tc.value, func(t *testing.T) {
			root := seed(t)
			options := ""
			if tc.typ == "enum" {
				options = ", options: [a, b]"
			}
			write(t, root, ConfigPath, "version: 1\nprefix: changed\nfields: {value: {type: "+tc.typ+options+"}}\n")
			write(t, root, ".wrk/old-00000001.md", source("old-00000001", "fields: {value: "+tc.value+", opaque: !unknown [one, two]}\n"))
			ds := Load(root).Diagnostics
			if (len(ds) == 0) != tc.valid {
				t.Fatalf("valid %v: %+v", tc.valid, ds)
			}
		})
	}
}
func TestGraphsAndWorkflow(t *testing.T) {
	root := seed(t)
	write(t, root, ".wrk/wrk-00000001.md", source("wrk-00000001", "depends_on: [wrk-00000002]\n"))
	write(t, root, ".wrk/wrk-00000002.md", source("wrk-00000002", "parent: wrk-00000001\n"))
	s := Load(root)
	if len(s.Diagnostics) > 0 {
		t.Fatal(s.Diagnostics)
	}
	if len(s.List(false, true)) != 1 || len(s.Blockers(s.ByID["wrk-00000001"])) != 1 {
		t.Fatal("wrong readiness")
	}
	for _, status := range []string{"todo", "in-progress", "canceled", "done"} {
		data := strings.Replace(source("wrk-00000002", "parent: wrk-00000001\n"), "status: todo", "status: "+status, 1)
		write(t, root, ".wrk/wrk-00000002.md", data)
		s = Load(root)
		want := 1
		if status == "done" {
			want = 0
		}
		if len(s.Blockers(s.ByID["wrk-00000001"])) != want {
			t.Fatal(status)
		}
	}
	for _, tc := range []struct{ extra, code string }{
		{"parent: wrk-00000002\n", "CYCLE"}, {"depends_on: [wrk-00000001]\n", "SELF_REFERENCE"}, {"depends_on: [wrk-00000002, wrk-00000002]\n", "DUPLICATE_DEPENDENCY"}, {"parent: wrk-00000009\n", "MISSING_REFERENCE"},
	} {
		write(t, root, ".wrk/wrk-00000001.md", source("wrk-00000001", tc.extra))
		if !hasCode(Load(root).Diagnostics, tc.code) {
			t.Fatalf("missing %s", tc.code)
		}
	}
	write(t, root, ".wrk/wrk-00000001.md", source("wrk-00000001", "depends_on: [wrk-00000002]\n"))
	write(t, root, ".wrk/wrk-00000002.md", source("wrk-00000002", "depends_on: [wrk-00000001]\n"))
	if !hasCode(Load(root).Diagnostics, "CYCLE") {
		t.Fatal("missed dependency cycle")
	}
}
func TestDeepHierarchy(t *testing.T) {
	root := seed(t)
	for i := 0; i < 150; i++ {
		id := fmt.Sprintf("wrk-%08x", i)
		extra := ""
		if i > 0 {
			extra = fmt.Sprintf("parent: wrk-%08x\n", i-1)
		}
		write(t, root, ".wrk/"+id+".md", source(id, extra))
	}
	if ds := Load(root).Diagnostics; len(ds) > 0 {
		t.Fatal(ds)
	}
}
func TestSnapshotComparison(t *testing.T) {
	for _, kind := range []string{"target", "config", "other", "add", "delete", "rename", "mode", "same-time"} {
		t.Run(kind, func(t *testing.T) {
			root := seed(t)
			path := ".wrk/wrk-00000001.md"
			write(t, root, path, source("wrk-00000001", ""))
			write(t, root, ".wrk/wrk-00000002.md", source("wrk-00000002", ""))
			s := Load(root)
			if err := s.Compare(); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "target", "same-time":
				info, _ := os.Stat(filepath.Join(root, path))
				data := bytes.Replace(s.Files[path].Data, []byte("Title"), []byte("Other"), 1)
				os.WriteFile(filepath.Join(root, path), data, 0644)
				if kind == "same-time" {
					os.Chtimes(filepath.Join(root, path), info.ModTime(), info.ModTime())
				}
			case "config":
				write(t, root, ConfigPath, strings.Replace(DefaultConfig, "wrk", "new", 1))
			case "other":
				write(t, root, ".wrk/wrk-00000002.md", source("wrk-00000002", "labels: [changed]\n"))
			case "add":
				write(t, root, ".wrk/wrk-00000003.md", source("wrk-00000003", ""))
			case "delete":
				os.Remove(filepath.Join(root, path))
			case "rename":
				write(t, root, ".wrk/.wrk-stage-editor", string(s.Files[path].Data))
				os.Rename(filepath.Join(root, ".wrk/.wrk-stage-editor"), filepath.Join(root, path))
			case "mode":
				os.Chmod(filepath.Join(root, path), 0600)
			}
			if err := s.Compare(); err == nil {
				t.Fatal("missed change")
			}
		})
	}
}
