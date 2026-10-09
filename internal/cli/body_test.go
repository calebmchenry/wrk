package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wrk/internal/project"
	"wrk/internal/store"
)

func TestUpdateBodyInputBeforeLock(t *testing.T) {
	root := t.TempDir()
	if m := store.Init(root); len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	const id = "wrk-12345678"
	if err := os.WriteFile(filepath.Join(root, ".wrk", id+".md"), []byte("---\nid: "+id+"\ntitle: T\nstatus: todo\n---\nOld"), 0600); err != nil {
		t.Fatal(err)
	}
	input := checkReader{check: func() {
		if _, err := os.Lstat(filepath.Join(root, ".wrk/.lock")); !os.IsNotExist(err) {
			t.Fatal("lock created before reading body")
		}
	}, reader: strings.NewReader("New 🦊\r\nbody")}
	var out, errout bytes.Buffer
	if code := Run([]string{"update", id, "--body-file=-", "--json"}, root, input, &out, &errout); code != 0 {
		t.Fatalf("%d: %s %s", code, out.String(), errout.String())
	}
	if got := project.Load(root).ByID[id]; string(got.Body) != "New 🦊\r\nbody" || got.Title != "T" {
		t.Fatal(got)
	}
}

type bodyErrorReader struct{}

func (bodyErrorReader) Read([]byte) (int, error) { return 0, errors.New("input unavailable") }

func TestUpdateRejectsBadBodyBeforeProjectAccess(t *testing.T) {
	for _, tc := range []struct {
		name, file, code string
		input            io.Reader
	}{
		{"invalid UTF-8", "-", "INVALID_BODY", bytes.NewReader([]byte{255})},
		{"stdin error", "-", "IO", bodyErrorReader{}},
		{"missing file", "missing.txt", "IO", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store.Init(root)
			var out, errout bytes.Buffer
			code := Run([]string{"update", "wrk-12345678", "--body-file", tc.file, "--json"}, root, tc.input, &out, &errout)
			var e Envelope
			if err := json.Unmarshal(out.Bytes(), &e); err != nil {
				t.Fatal(err)
			}
			if code != 1 || e.Result != nil || e.Errors[0].Code != tc.code || errout.Len() != 0 {
				t.Fatalf("%d: %s %s", code, out.String(), errout.String())
			}
			if _, err := os.Lstat(filepath.Join(root, ".wrk/.lock")); !os.IsNotExist(err) {
				t.Fatal("bad body acquired lock")
			}
		})
	}
}

func TestUpdateBodyArguments(t *testing.T) {
	for _, args := range [][]string{
		{"update", "id", "--body-file=-"},
		{"update", "--body-file", "input.md", "id", "--status=done"},
	} {
		if _, err := Parse(args); err != nil {
			t.Fatal(args, err)
		}
	}
	for _, args := range [][]string{
		{"update", "id", "--body-file"},
		{"update", "id", "--body-file=a", "--body-file=b"},
		{"update", "id", "--recursive", "--body-file=-"},
		{"update", "id", "--recursive", "--label=x", "--body-file=-"},
		{"list", "--body-file=-"},
	} {
		if _, err := Parse(args); err == nil {
			t.Fatal("accepted", args)
		}
	}
}
