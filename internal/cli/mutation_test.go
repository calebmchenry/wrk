package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"wrk/internal/diagnostic"
	"wrk/internal/project"
	"wrk/internal/store"
)

type checkReader struct {
	check  func()
	reader io.Reader
}

func (r checkReader) Read(p []byte) (int, error) { r.check(); return r.reader.Read(p) }

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestBodyIsReadBeforeLock(t *testing.T) {
	root := t.TempDir()
	if r := store.Init(root); len(r.Diagnostics) > 0 {
		t.Fatal(r.Diagnostics)
	}
	input := checkReader{check: func() {
		if _, err := os.Lstat(filepath.Join(root, ".wrk/.lock")); !os.IsNotExist(err) {
			t.Fatal("lock created before body read")
		}
	}, reader: bytes.NewBufferString("Body 🦊")}
	var out, errout bytes.Buffer
	if code := Run([]string{"new", "Title", "--body-file=-", "--json"}, root, input, &out, &errout); code != 0 {
		t.Fatalf("%s %s", out.String(), errout.String())
	}
}
func TestInvalidBodyAndFailedOutput(t *testing.T) {
	root := t.TempDir()
	store.Init(root)
	var out, errout bytes.Buffer
	if code := Run([]string{"new", "Title", "--body-file=-", "--json"}, root, bytes.NewReader([]byte{255}), &out, &errout); code != 1 {
		t.Fatal(code)
	}
	if _, err := os.Lstat(filepath.Join(root, ".wrk/.lock")); !os.IsNotExist(err) {
		t.Fatal("invalid body created lock")
	}
	if code := Run([]string{"new", "Committed", "--json"}, root, nil, failingWriter{}, &errout); code != 1 {
		t.Fatal(code)
	}
	if s := project.Load(root); len(s.Diagnostics) > 0 || len(s.Tickets) != 1 {
		t.Fatal("output error rolled back committed data")
	}
}
func TestCommittedErrorJSON(t *testing.T) {
	root := t.TempDir()
	store.Init(root)
	m := store.Create(root, store.CreateOptions{Title: "Committed"})
	if len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	m.Diagnostics = append(m.Diagnostics, diagnostic.New("CLEANUP_FAILED", "injected cleanup failure", m.Ticket.Path))
	r := Request{Command: "new", JSON: true}
	e := Envelope{SchemaVersion: 1, Command: "new", ProjectRoot: &root}
	var out, errout bytes.Buffer
	if code := renderMutation(&out, &errout, r, e, m); code != 1 || errout.Len() > 0 {
		t.Fatalf("%d %s", code, errout.String())
	}
	var got struct {
		OK     bool `json:"ok"`
		Result struct {
			Publication string          `json:"publication"`
			Ticket      project.Summary `json:"ticket"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.OK || got.Result.Publication != "committed" || got.Result.Ticket.ID != m.Ticket.ID {
		t.Fatalf("%s", out.String())
	}
}
