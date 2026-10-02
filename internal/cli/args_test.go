package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestArguments(t *testing.T) {
	good := [][]string{nil, {"--json", "list", "--ready"}, {"new", "Title", "--label=x", "--label", "y"}, {"update", "--status=done", "id", "--title", "T"}, {"new", "--", "--title"}, {"help", "show"}, {"list", "--help"}, {"init"}}
	for _, a := range good {
		if _, err := Parse(a); err != nil {
			t.Errorf("%v: %v", a, err)
		}
	}
	bad := [][]string{{"list", "--all", "--ready"}, {"new", "T", "--no-labels", "--label=x"}, {"show"}, {"show", "a", "b"}, {"update", "a"}, {"update", "a", "--title=a", "--title=b"}, {"list", "--wat"}, {"--json", "--json"}, {"new", "T", "--parent"}, {"list", "--all=true"}, {"help", "nope"}}
	for _, a := range bad {
		if _, err := Parse(a); err == nil {
			t.Errorf("accepted %v", a)
		}
	}
}
func TestHelpAndErrorEnvelopes(t *testing.T) {
	for _, args := range [][]string{{"--json"}, {"list", "--wat", "--json"}} {
		var out, errout bytes.Buffer
		code := Run(args, t.TempDir(), nil, &out, &errout)
		var e Envelope
		if err := json.Unmarshal(out.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		if e.SchemaVersion != 1 || e.ProjectRoot != nil || errout.Len() != 0 {
			t.Fatalf("bad envelope: %+v stderr %s", e, errout.String())
		}
		if args[0] == "list" && code != 2 {
			t.Fatal(code)
		}
	}
}

func TestReadJSON(t *testing.T) {
	root, err := filepath.Abs("../../testdata/compat")
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"validate", "--json"}, {"list", "--all", "--json"}, {"show", "wrk-682f60c7", "--json"}, {"show", "wrk-deadbeef", "--json"}} {
		var out, errout bytes.Buffer
		code := Run(args, root, nil, &out, &errout)
		var e Envelope
		if err := json.Unmarshal(out.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		if errout.Len() > 0 || e.ProjectRoot == nil || *e.ProjectRoot != root || e.Errors == nil {
			t.Fatalf("bad result: %s %s", out.String(), errout.String())
		}
		if args[1] == "wrk-deadbeef" {
			if code != 1 || e.OK {
				t.Fatal("missing ticket succeeded")
			}
		} else if code != 0 || !e.OK {
			t.Fatalf("%+v", e)
		}
	}
}
