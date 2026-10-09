package cli

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
)

func TestMetadataArguments(t *testing.T) {
	for _, args := range [][]string{
		{"update", "id", "--priority=high"},
		{"update", "--label=z", "id", "--label", "a", "--label=a"},
		{"update", "id", "--no-labels"},
		{"update", "id", "--priority=low", "--no-labels", "--title=T", "--status=todo"},
		{"update", "id", "--priority=urgent", "--add-label=a", "--remove-label=b"},
		{"update", "id", "--label=--dash", "--recursive"},
		{"update", "id", "--no-labels", "--recursive"},
	} {
		if _, err := Parse(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	r, err := Parse([]string{"update", "id", "--label=z", "--label=a", "--label=a", "--priority=high"})
	if err != nil || !r.changes(nil).LabelsSet || !slices.Equal(r.changes(nil).Labels, []string{"z", "a", "a"}) || *r.changes(nil).Priority != "high" {
		t.Fatalf("%+v %v", r, err)
	}
	bad := [][]string{
		{"--label=a", "--no-labels"}, {"--no-labels", "--label=a"},
		{"--label=a", "--add-label=b"}, {"--add-label=b", "--label=a"},
		{"--label=a", "--remove-label=b"}, {"--remove-label=b", "--label=a"},
		{"--no-labels", "--add-label=b"}, {"--add-label=b", "--no-labels"},
		{"--no-labels", "--remove-label=b"}, {"--remove-label=b", "--no-labels"},
		{"--priority=high", "--priority=low"}, {"--no-labels", "--no-labels"},
		{"--recursive", "--priority=normal", "--no-labels"},
		{"--recursive", "--priority=high", "--label=a"},
		{"--recursive", "--title=T", "--no-labels"},
		{"--recursive", "--status=todo", "--label=a"},
		{"--no-labels=true"}, {"--priority"}, {"--label"}, {"--label="}, {"--label=\xff"},
	}
	for _, flags := range bad {
		var out, errout bytes.Buffer
		args := append([]string{"update", "id", "--json"}, flags...)
		if code := Run(args, t.TempDir(), nil, &out, &errout); code != 2 {
			t.Fatalf("%v: %d %s", flags, code, out.String())
		}
		var e Envelope
		if err := json.Unmarshal(out.Bytes(), &e); err != nil || e.Result != nil || e.ProjectRoot != nil || e.Errors[0].Code != "USAGE" || errout.Len() != 0 {
			t.Fatalf("%s %v", out.String(), err)
		}
	}
}
