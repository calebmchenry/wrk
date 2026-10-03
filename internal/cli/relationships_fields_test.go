package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRelationshipAndFieldArgumentContracts(t *testing.T) {
	id := "wrk-12345678"
	for _, args := range [][]string{
		{"new", "Title", "--depends-on", id, "--field=estimate=2", "--field", "review=true"},
		{"update", id, "--parent=" + id, "--add-dependency=" + id, "--add-dependency=" + id},
		{"update", id, "--no-parent", "--remove-dependency=" + id, "--remove-field=x", "--remove-field=x"},
		{"update", id, "--field", "x=&loop [*loop]", "--field=y='true'", "--status=done"},
	} {
		if _, err := Parse(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"update", id, "--parent=" + id, "--no-parent"},
		{"update", id, "--parent=" + id, "--parent=" + id},
		{"update", id, "--no-parent=true"},
		{"update", id, "--no-parent", "--no-parent"},
		{"update", id, "--add-dependency=" + id, "--remove-dependency=" + id},
		{"update", id, "--remove-dependency="},
		{"update", id, "--field=x=1", "--field=x=1"},
		{"new", "Title", "--field=x=1", "--field=x=2"},
		{"update", id, "--field=x=1", "--remove-field=x"},
		{"update", id, "--remove-field=x", "--field=x=1"},
		{"update", id, "--field=x="},
		{"update", id, "--remove-field="},
		{"update", id, "--remove-field=\xff"},
		{"update", id, "--field=x={a: 1, a: 2}"},
		{"update", id, "--field=x=1\n---\n2"},
		{"update", id, "--field=x=*missing"},
		{"update", id, "--parent"},
		{"update", id, "--add-dependency"},
		{"new", "Title", "--remove-field=x"},
		{"new", "Title", "--add-dependency=" + id},
		{"update", id, "--depends-on=" + id},
	} {
		var out, errout bytes.Buffer
		args = append(args, "--json")
		if code := Run(args, t.TempDir(), nil, &out, &errout); code != 2 {
			t.Fatalf("%v: %d %s", args, code, out.String())
		}
		var e Envelope
		if err := json.Unmarshal(out.Bytes(), &e); err != nil || e.Result != nil || e.ProjectRoot != nil || len(e.Errors) != 1 || e.Errors[0].Code != "USAGE" || errout.Len() != 0 {
			t.Fatalf("%v: %s %v", args, out.String(), err)
		}
	}
	for _, flag := range []string{"--parent=" + id, "--no-parent", "--add-dependency=" + id, "--remove-dependency=" + id, "--field=x=1", "--remove-field=x"} {
		if _, err := Parse([]string{"update", id, "--recursive", "--add-label=batch", flag}); err == nil {
			t.Fatal("recursive accepted", flag)
		}
	}
}
