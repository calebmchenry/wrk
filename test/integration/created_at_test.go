package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreatedAtCLI(t *testing.T) {
	root := t.TempDir()
	run(t, root, "", 0, "init", "--json")
	legacy := "wrk-00000001"
	legacySource := "---\nid: " + legacy + "\ntitle: Legacy\nstatus: todo\n---\nbody"
	put(t, root, ".wrk/"+legacy+".md", legacySource)
	start := time.Now().UTC().Truncate(time.Second)
	created := result(t, run(t, root, "description", 0, "new", "New child", "--parent", legacy, "--priority", "high", "--label", "tag", "--depends-on", legacy, "--related", legacy, "--field", "size=3", "--body-file=-", "--json"))["ticket"].(map[string]any)
	id := created["id"].(string)
	date, ok := created["created_at"].(string)
	parsed, err := time.Parse(time.RFC3339, date)
	if !ok || err != nil || !strings.HasSuffix(date, "Z") || parsed.Before(start) || parsed.After(time.Now()) {
		t.Fatalf("creation time: %v (%v)", created, err)
	}
	assertDate := func(summary map[string]any, want any) {
		t.Helper()
		if got, exists := summary["created_at"]; !exists || got != want {
			t.Fatalf("created_at = %v, exists=%t, want %v", got, exists, want)
		}
	}
	for _, args := range [][]string{
		{"update", id, "--title", "Changed", "--status", "done", "--priority", "low", "--no-labels", "--body-file=-", "--field", "size=5"},
		{"update", id, "--no-parent", "--remove-dependency", legacy},
		{"update", id, "--parent", legacy, "--status", "todo"},
		{"update", legacy, "--remove-related", id}, // Reverse link removal rewrites the timestamped owner.
		{"update", legacy, "--add-label", "recursive", "--recursive"},
		{"update", legacy, "--body-file=-", "--status", "done"},
	} {
		run(t, root, "new body", 0, append(args, "--json")...)
		assertDate(result(t, run(t, root, "", 0, "show", id, "--json"))["ticket"].(map[string]any), date)
		assertDate(result(t, run(t, root, "", 0, "show", legacy, "--json"))["ticket"].(map[string]any), nil)
	}
	parent := result(t, run(t, root, "", 0, "show", legacy, "--json"))
	assertDate(parent["children"].([]any)[0].(map[string]any), date)
	for _, entry := range result(t, run(t, root, "", 0, "list", "--all", "--json"))["tickets"].([]any) {
		summary := entry.(map[string]any)
		var want any
		if summary["id"] == id {
			want = date
		}
		assertDate(summary, want)
	}
	for _, args := range [][]string{{"list", "--all"}, {"show", id}, {"new", "Human creation"}} {
		cmd := exec.Command(binary, args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		if args[0] == "new" {
			humanID := strings.Fields(string(out))[0]
			humanDate := result(t, run(t, root, "", 0, "show", humanID, "--json"))["ticket"].(map[string]any)["created_at"].(string)
			if !strings.Contains(string(out), "(created "+humanDate+")") {
				t.Fatal(string(out))
			}
		} else if !strings.Contains(string(out), date) || (args[0] == "list" && (!strings.Contains(string(out), "CREATED_AT") || !strings.Contains(string(out), "unknown"))) {
			t.Fatal(string(out))
		}
	}
	for _, args := range [][]string{{"new", "Override", "--created-at", date}, {"update", id, "--created-at", date}} {
		run(t, root, "", 2, append(args, "--json")...)
	}
	source, err := os.ReadFile(filepath.Join(root, ".wrk", id+".md"))
	if err != nil || !strings.Contains(string(source), "created_at: "+date+"\n") {
		t.Fatal(string(source), err)
	}
	legacyBytes, err := os.ReadFile(filepath.Join(root, ".wrk", legacy+".md"))
	if err != nil || strings.Contains(string(legacyBytes), "created_at:") {
		t.Fatal(string(legacyBytes), err)
	}
	run(t, root, "", 0, "validate", "--json")
}
