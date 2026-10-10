//go:build darwin || linux

package integration

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A fake Codex on PATH checks the documented argv, then mutates real disposable
// tickets through the compiled CLI. Never fall back to an installed AI client.
func TestRunCodexBurn(t *testing.T) {
	for _, mode := range []string{"done", "failure", "unfinished"} {
		t.Run(mode, func(t *testing.T) {
			root, ids := runProject(t, 2)
			launch, _ := runProject(t, 0)
			// Completing the later ID must reveal the earlier dependent ticket.
			run(t, root, "", 0, "update", ids[0], "--add-dependency="+ids[1], "--json")
			if err := os.Mkdir(filepath.Join(root, "tools"), 0700); err != nil {
				t.Fatal(err)
			}
			put(t, root, "tools/codex", `#!/bin/sh
set -eu
test "$#" = 3
test "$1" = exec
test "$2" = --dangerously-bypass-approvals-and-sandbox
id=${3#Implement }
test "$3" = "Implement $id"
printf '%s\n' "$id" >> actions
printf 'fake Codex stdout\n'
printf 'fake Codex stderr\n' >&2
"$WRK_BURN_TEST_BINARY" update "$id" --status in-progress
case "$WRK_BURN_TEST_MODE" in
  failure) exit 7 ;;
  unfinished) exit 0 ;;
esac
"$WRK_BURN_TEST_BINARY" update "$id" --status done
`)
			if err := os.Chmod(filepath.Join(root, "tools/codex"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", filepath.Join(root, "tools"))
			t.Setenv("WRK_BURN_TEST_BINARY", binary)
			t.Setenv("WRK_BURN_TEST_MODE", mode)
			wantExit := 1
			if mode == "done" {
				wantExit = 0
			}
			r, _, _ := runAction(t, launch, wantExit,
				[]string{"--project", root, "--ready", "--label=batch", "--expect-status=done"},
				[]string{"codex", "exec", "--dangerously-bypass-approvals-and-sandbox", "Implement {id}"})
			wantIDs := []string{ids[1]}
			if mode == "done" {
				wantIDs = append(wantIDs, ids[0])
				if r.Successful != 2 || r.Reason != "drained" || r.LastAction.ObservedStatus != "done" {
					t.Fatalf("%+v", r)
				}
			} else {
				kind, exit := "completion", 0
				if mode == "failure" {
					kind, exit = "exit", 7
				}
				if r.Successful != 0 || r.Failure.Kind != kind || *r.LastAction.ExitCode != exit || r.LastAction.ObservedStatus != "in-progress" {
					t.Fatalf("%+v", r)
				}
			}
			if !reflect.DeepEqual(actionIDs(t, root), wantIDs) || len(actionIDs(t, launch)) != 0 {
				t.Fatalf("actions: %v want %v", actionIDs(t, root), wantIDs)
			}
		})
	}
}

func TestRunMultiStepExplicitRetry(t *testing.T) {
	root, ids := runProject(t, 2)
	launch, _ := runProject(t, 0)
	t.Setenv("WRK_BURN_TEST_BINARY", binary)
	t.Setenv("WRK_BURN_TEST_VALUE", "exported value")
	if err := os.Mkdir(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	// Non-executable script with spaces requires an explicit interpreter. All
	// its relative paths must resolve in the selected project, not launch cwd.
	put(t, root, "scripts/two steps.sh", `set -eu
id=$1
test "$WRK_BURN_TEST_VALUE" = 'exported value'
"$WRK_BURN_TEST_BINARY" update "$id" --status in-progress
printf '%s\n' "$id" >> actions
printf 'step one\n' >> attempts
if test -f durable-result; then
  IFS= read -r result < durable-result
  test "$result" = "result for $id"
else
  printf 'result for %s\n' "$id" > durable-result
fi
printf 'step two\n' >> attempts
if ! test -f allow-step-two; then
  printf 'second step needs repair\n' >&2
  exit 7
fi
printf 'workflow complete\n' > completed-result
"$WRK_BURN_TEST_BINARY" update "$id" --status done
`)
	action := []string{"/bin/sh", "scripts/two steps.sh", "{id}"}
	r, _, _ := runAction(t, launch, 1,
		[]string{"--project", root, "--ready", "--expect-status=done"}, action)
	if r.Successful != 0 || r.Failure.Kind != "exit" || r.LastAction.ObservedStatus != "in-progress" || !reflect.DeepEqual(actionIDs(t, root), ids[:1]) {
		t.Fatalf("%+v", r)
	}
	before, err := os.Stat(filepath.Join(root, "durable-result"))
	if err != nil {
		t.Fatal(err)
	}
	ready := result(t, run(t, root, "", 0, "list", "--ready", "--json"))["tickets"].([]any)
	if len(ready) != 1 || ready[0].(map[string]any)["id"] != ids[1] {
		t.Fatalf("failed in-progress ticket must not be ready: %v", ready)
	}
	if _, err := os.Stat(filepath.Join(root, "completed-result")); !os.IsNotExist(err) {
		t.Fatal("step two completed despite failure")
	}
	// Inspection/repair is external; there is no runner checkpoint or retry.
	put(t, root, "allow-step-two", "repaired\n")
	r, _, _ = runAction(t, launch, 0,
		[]string{"--config", filepath.Join(root, ".wrk/config.yaml"), "--ticket=" + ids[0], "--expect-status=done"}, action)
	if r.Successful != 1 || r.Reason != "ticket" || r.LastAction.ObservedStatus != "done" || !reflect.DeepEqual(actionIDs(t, root), []string{ids[0], ids[0]}) {
		t.Fatalf("%+v", r)
	}
	attempts, err := os.ReadFile(filepath.Join(root, "attempts"))
	if err != nil || string(attempts) != strings.Repeat("step one\nstep two\n", 2) {
		t.Fatalf("whole workflow must restart: %q %v", attempts, err)
	}
	after, err := os.Stat(filepath.Join(root, "durable-result"))
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("retry should reuse the durable first result: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(root, "completed-result")); err != nil || string(body) != "workflow complete\n" {
		t.Fatalf("missing final result: %q %v", body, err)
	}
}
