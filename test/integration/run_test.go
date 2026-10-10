//go:build darwin || linux

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"wrk/internal/runner"
)

// TestRunChild is an actual external program used by the compiled CLI. The
// explicit delimiter prevents ordinary test runs from performing helper actions.
func TestRunChild(t *testing.T) {
	i := slices.Index(os.Args, "--")
	if i < 0 {
		return
	}
	args := os.Args[i+1:]
	mode, id := args[0], args[1]
	if mode == "descendant" {
		signal.Ignore(syscall.SIGTERM, os.Interrupt)
		if err := os.WriteFile("descendant.pid", []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(90)
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	f, err := os.OpenFile("actions", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(91)
	}
	fmt.Fprintln(f, id)
	f.Close()
	cli := func(argv ...string) {
		cmd := exec.Command(os.Getenv("WRK_RUN_TEST_BINARY"), argv...)
		if data, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "child CLI: %v: %s", err, data)
			os.Exit(92)
		}
	}
	switch mode {
	case "noisy":
		for range 1024 {
			os.Stdout.Write([]byte(strings.Repeat("out{}\x00\x1b\n", 1024)))
			os.Stderr.Write([]byte(strings.Repeat("err{}\x00\x1b\n", 1024)))
		}
	case "gated":
		waitForFile := func(name string) {
			deadline := time.Now().Add(8 * time.Second)
			for {
				if _, err := os.Stat(name); err == nil {
					return
				}
				if time.Now().After(deadline) {
					os.Exit(96)
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
		waitForFile("emit-output")
		fmt.Fprint(os.Stdout, "live stdout")
		fmt.Fprint(os.Stderr, "live stderr")
		waitForFile("finish-action")
	case "noop":
	case "done":
		cli("update", id, "--status=done")
	case "fail", "mismatch":
		cli("update", id, "--status=in-progress")
		if mode == "fail" {
			os.Exit(7)
		}
	case "delete":
		os.Remove(filepath.Join(".wrk", id+".md"))
	case "invalid":
		os.WriteFile(filepath.Join(".wrk", id+".md"), []byte("---\ninvalid yaml [\n---\n"), 0600)
	case "add":
		if _, err := os.Stat("added"); os.IsNotExist(err) {
			cli("new", "Added during action", "--label=batch")
			os.WriteFile("added", nil, 0600)
		}
	case "capture":
		input, err := io.ReadAll(os.Stdin)
		if err != nil || len(input) != 0 {
			os.Exit(93)
		}
		cwd, _ := os.Getwd()
		data, _ := json.Marshal(map[string]any{"args": args[2:], "cwd": cwd, "env": os.Getenv("WRK_RUN_TEST_VALUE"), "path": os.Getenv("PATH"), "dotenv": os.Getenv("WRK_RUN_TEST_DOTENV")})
		os.WriteFile("capture.json", data, 0600)
		fmt.Fprintln(os.Stdout, "child stdout")
		fmt.Fprintln(os.Stderr, "child stderr")
	case "interrupt", "interrupt-early":
		if mode == "interrupt" {
			signal.Ignore(syscall.SIGTERM, os.Interrupt)
		}
		os.WriteFile("child.pid", []byte(strconv.Itoa(os.Getpid())), 0600)
		self, _ := os.Executable()
		cmd := exec.Command(self, "-test.run=^TestRunChild$", "--", "descendant", id)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			os.Exit(94)
		}
		cli("update", id, "--status=in-progress")
		os.WriteFile("partial-change", []byte("retained"), 0600)
		for {
			time.Sleep(time.Hour)
		}
	default:
		os.Exit(95)
	}
	os.Exit(0)
}

func actionCommand(t *testing.T, mode string, extra ...string) []string {
	t.Helper()
	t.Setenv("WRK_RUN_TEST_HELPER", "1")
	t.Setenv("WRK_RUN_TEST_BINARY", binary)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{self, "-test.run=^TestRunChild$", "--", mode, "{id}"}, extra...)
}

func runAction(t *testing.T, cwd string, want int, options, action []string) (runner.Result, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	argv := append([]string{"run", "--json"}, options...)
	argv = append(append(argv, "--"), action...)
	cmd := exec.CommandContext(ctx, binary, argv...)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader("must not reach child")
	var out, errout bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errout
	err := cmd.Run()
	code := 0
	if err != nil {
		var ok bool
		if exit, yes := err.(*exec.ExitError); yes {
			code, ok = exit.ExitCode(), true
		}
		if !ok {
			t.Fatal(err)
		}
	}
	if code != want {
		t.Fatalf("exit %d want %d: %s\n%s", code, want, out.String(), errout.String())
	}
	e := runFinalEnvelope(t, out.Bytes())
	if e.OK != (want == 0) || e.Command != "run" || e.Root == nil {
		t.Fatalf("bad envelope %+v", e)
	}
	var result runner.Result
	var event struct {
		Outcome runner.Result `json:"outcome"`
	}
	if err := json.Unmarshal(e.Result, &event); err != nil {
		t.Fatal(err)
	}
	result = event.Outcome
	return result, out.String(), errout.String()
}

func runProject(t *testing.T, n int) (string, []string) {
	t.Helper()
	root := t.TempDir()
	run(t, root, "", 0, "init", "--json")
	ids := []string{}
	for i := 0; i < n; i++ {
		ids = append(ids, newID(t, run(t, root, "", 0, "new", fmt.Sprintf("Ticket %d", i), "--label=batch", "--json")))
	}
	slices.Sort(ids)
	return root, ids
}

func actionIDs(t *testing.T, root string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "actions"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}

func TestRunSelectionAndRequery(t *testing.T) {
	t.Run("unchanged active statuses once and fresh invocation", func(t *testing.T) {
		root, ids := runProject(t, 5)
		for i, status := range []string{"todo", "in-progress", "blocked", "done", "canceled"} {
			run(t, root, "", 0, "update", ids[i], "--status="+status, "--json")
		}
		action := actionCommand(t, "noop")
		result, _, _ := runAction(t, root, 0, nil, action)
		if result.Successful != 3 || result.Reason != "drained" || !reflect.DeepEqual(actionIDs(t, root), ids[:3]) {
			t.Fatalf("%+v %v", result, actionIDs(t, root))
		}
		result, _, _ = runAction(t, root, 0, []string{"--all"}, action)
		if result.Successful != 5 || !reflect.DeepEqual(actionIDs(t, root), append(slices.Clone(ids[:3]), ids...)) {
			t.Fatalf("%+v %v", result, actionIDs(t, root))
		}
	})
	t.Run("dependencies unlock in fresh ID order", func(t *testing.T) {
		root, ids := runProject(t, 3)
		// The lowest ID is blocked by the highest; it must run before the middle
		// ID after the first action unlocks both.
		for _, id := range ids[:2] {
			run(t, root, "", 0, "update", id, "--add-dependency="+ids[2], "--json")
		}
		result, _, _ := runAction(t, root, 0, []string{"--ready", "--expect-status=done"}, actionCommand(t, "done"))
		want := []string{ids[2], ids[0], ids[1]}
		if result.Successful != 3 || !reflect.DeepEqual(actionIDs(t, root), want) {
			t.Fatalf("%+v %v", result, actionIDs(t, root))
		}
	})
	t.Run("new matching ticket", func(t *testing.T) {
		root, _ := runProject(t, 1)
		result, _, _ := runAction(t, root, 0, []string{"--label=batch"}, actionCommand(t, "add"))
		if result.Successful != 2 || len(actionIDs(t, root)) != 2 {
			t.Fatalf("%+v", result)
		}
	})
	t.Run("AND labels descendants root exclusion", func(t *testing.T) {
		root, ids := runProject(t, 4)
		for _, id := range ids[:3] {
			run(t, root, "", 0, "update", id, "--add-label=second", "--json")
		}
		for i := 1; i < 4; i++ {
			run(t, root, "", 0, "update", ids[i], "--parent="+ids[i-1], "--json")
		}
		result, _, _ := runAction(t, root, 0, []string{"--under=" + ids[0], "--label=batch", "--label=second"}, actionCommand(t, "noop"))
		if result.Successful != 2 || !reflect.DeepEqual(actionIDs(t, root), ids[1:3]) {
			t.Fatalf("%+v %v", result, actionIDs(t, root))
		}
	})
	t.Run("positive limit counts unchanged successes", func(t *testing.T) {
		root, ids := runProject(t, 3)
		result, _, _ := runAction(t, root, 0, []string{"--max-tickets=2"}, actionCommand(t, "noop"))
		if result.Successful != 2 || result.Reason != "limit" || !reflect.DeepEqual(actionIDs(t, root), ids[:2]) {
			t.Fatalf("%+v", result)
		}
	})
	t.Run("empty query", func(t *testing.T) {
		root, _ := runProject(t, 0)
		result, _, _ := runAction(t, root, 0, nil, []string{"missing-child"})
		if result.Successful != 0 || result.LastAction != nil || result.Reason != "drained" {
			t.Fatalf("%+v", result)
		}
	})
}

func TestRunFailureAndRetry(t *testing.T) {
	t.Run("missing command is usage error", func(t *testing.T) {
		root, _ := runProject(t, 1)
		for _, args := range [][]string{{"run", "--json"}, {"run", "--json", "--"}, {"run", "--json", "--", ""}} {
			run(t, root, "", 2, args...)
		}
	})
	for _, mode := range []string{"fail", "mismatch", "delete", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			root, ids := runProject(t, 2)
			result, _, _ := runAction(t, root, 1, []string{"--ready", "--expect-status=done", "--max-tickets=1"}, actionCommand(t, mode))
			if result.Successful != 0 || result.LastAction.Ticket.ID != ids[0] || !reflect.DeepEqual(actionIDs(t, root), ids[:1]) {
				t.Fatalf("%+v", result)
			}
			if mode == "fail" {
				if result.Failure.Kind != "exit" || *result.LastAction.ExitCode != 7 {
					t.Fatalf("%+v", result)
				}
				// Readiness no longer includes this ticket. Explicit retry does not
				// reset its status and reruns the supplied action from the beginning.
				result, _, _ = runAction(t, root, 0, []string{"--ticket=" + ids[0], "--expect-status=in-progress"}, actionCommand(t, "noop"))
				if result.Successful != 1 || result.Reason != "ticket" || !reflect.DeepEqual(actionIDs(t, root), []string{ids[0], ids[0]}) {
					t.Fatalf("%+v", result)
				}
			} else if result.Failure.Kind != "completion" {
				t.Fatalf("%+v", result)
			}
		})
	}
	for _, mode := range []string{"delete", "invalid"} {
		t.Run(mode+" without status requirement", func(t *testing.T) {
			root, _ := runProject(t, 2)
			result, _, _ := runAction(t, root, 1, nil, actionCommand(t, mode))
			if result.Successful != 0 || result.Failure.Kind != "completion" || len(actionIDs(t, root)) != 1 {
				t.Fatalf("%+v", result)
			}
		})
	}
	t.Run("malformed project", func(t *testing.T) {
		root, _ := runProject(t, 1)
		put(t, root, ".wrk/config.yaml", "invalid: [")
		result, _, _ := runAction(t, root, 1, nil, actionCommand(t, "noop"))
		if result.Failure.Kind != "project" || len(actionIDs(t, root)) != 0 {
			t.Fatalf("%+v", result)
		}
	})
	for _, options := range [][]string{{"--ticket=wrk-deadbeef"}, {"--under=wrk-deadbeef"}} {
		root, _ := runProject(t, 1)
		result, _, _ := runAction(t, root, 1, options, actionCommand(t, "noop"))
		if result.Failure.Kind != "selection" || result.LastAction != nil {
			t.Fatalf("%+v", result)
		}
	}
	t.Run("missing executable", func(t *testing.T) {
		root, ids := runProject(t, 2)
		result, _, _ := runAction(t, root, 1, nil, []string{"wrk-nonexistent-command"})
		if result.Failure.Kind != "start" || result.LastAction.ExitCode != nil || result.LastAction.Ticket.ID != ids[0] {
			t.Fatalf("%+v", result)
		}
	})
}

func TestRunHumanFailureIncludesRetry(t *testing.T) {
	root, ids := runProject(t, 2)
	action := actionCommand(t, "fail", "space and 'quote'", "$(literal)", "")
	argv := append([]string{"run", "--ready", "--expect-status=done", "--"}, action...)
	cmd := exec.Command(binary, argv...)
	cmd.Dir = root
	var out, errout bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errout
	if err := cmd.Run(); err == nil {
		t.Fatal("failure succeeded")
	}
	for _, value := range []string{"0 successful actions", "'--ticket' '" + ids[0] + "'", "'--expect-status' 'done'", "'--project' '" + root + "'", "'space and '\"'\"'quote'\"'\"''", "'$(literal)'", "''"} {
		if !strings.Contains(out.String(), value) {
			t.Errorf("missing %q in %s", value, out.String())
		}
	}
	for _, value := range []string{"Starting " + ids[0], "Finished " + ids[0], "exit 7", "RUN_EXIT"} {
		if !strings.Contains(errout.String(), value) {
			t.Errorf("missing %q in %s", value, errout.String())
		}
	}
	if !reflect.DeepEqual(actionIDs(t, root), ids[:1]) {
		t.Fatal("started second ticket")
	}
}

func TestRunArgumentsEnvironmentAndProject(t *testing.T) {
	for _, lookup := range []string{"relative", "absolute PATH", "relative PATH", "config"} {
		t.Run(lookup, func(t *testing.T) {
			root, ids := runProject(t, 1)
			launch, _ := runProject(t, 1)
			relativeRoot, err := filepath.Rel(launch, root)
			if err != nil {
				t.Fatal(err)
			}
			self, _ := os.Executable()
			// Braces in the executable name must remain literal.
			if err := os.Symlink(self, filepath.Join(root, "fake-{id}")); err != nil {
				t.Fatal(err)
			}
			action := actionCommand(t, "capture", "", "two words", " --flags ", "--json", "--project=elsewhere", "--", "{id}/{id}", "$(touch unwanted);*>|&", "a'b", "a\nb", "relative-file")
			action[0] = "./fake-{id}"
			options := []string{"--project", relativeRoot}
			switch lookup {
			case "absolute PATH":
				t.Setenv("PATH", root)
				action[0] = "fake-{id}"
			case "relative PATH":
				t.Setenv("PATH", ".")
				action[0] = "fake-{id}"
			case "config":
				options = []string{"--config", filepath.Join(relativeRoot, ".wrk/config.yaml")}
			}
			t.Setenv("WRK_RUN_TEST_VALUE", "exported value $x")
			t.Setenv("WRK_RUN_TEST_DOTENV", "")
			put(t, root, ".env", "WRK_RUN_TEST_DOTENV=must-not-load\n")
			result, _, stderr := runAction(t, launch, 0, append(options, "--verbose"), action)
			if result.Successful != 1 || !strings.Contains(stderr, "child stdout") || !strings.Contains(stderr, "child stderr") {
				t.Fatalf("%+v %s", result, stderr)
			}
			var capture struct {
				Args                   []string
				Cwd, Env, Path, Dotenv string
			}
			data, err := os.ReadFile(filepath.Join(root, "capture.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &capture); err != nil {
				t.Fatal(err)
			}
			want := slices.Clone(action[5:])
			for i := range want {
				want[i] = strings.ReplaceAll(want[i], "{id}", ids[0])
			}
			physicalRoot, _ := filepath.EvalSymlinks(root)
			physicalCwd, _ := filepath.EvalSymlinks(capture.Cwd)
			if !reflect.DeepEqual(capture.Args, want) || physicalCwd != physicalRoot || capture.Env != "exported value $x" || capture.Path != os.Getenv("PATH") || capture.Dotenv != "" {
				t.Fatalf("capture %+v want args %q", capture, want)
			}
			if len(actionIDs(t, launch)) != 0 {
				t.Fatal("executed in launch project")
			}
			if _, err := os.Stat(filepath.Join(root, "unwanted")); !os.IsNotExist(err) {
				t.Fatal("evaluated metacharacters")
			}
		})
	}
	t.Run("completion stays in selected project", func(t *testing.T) {
		root, ids := runProject(t, 1)
		launch, _ := runProject(t, 0)
		data, err := os.ReadFile(filepath.Join(root, ".wrk", ids[0]+".md"))
		if err != nil {
			t.Fatal(err)
		}
		put(t, launch, ".wrk/"+ids[0]+".md", strings.Replace(string(data), "status: todo", "status: done", 1))
		result, _, _ := runAction(t, launch, 1, []string{"--project", root, "--expect-status=done"}, actionCommand(t, "mismatch"))
		if result.LastAction.ObservedStatus != "in-progress" {
			t.Fatalf("%+v", result)
		}
	})
}

func TestRunInterruptProcessGroup(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		for _, mode := range []string{"interrupt", "interrupt-early"} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/stream=%t", sig, mode, stream), func(t *testing.T) {
					root, ids := runProject(t, 2)
					argv := []string{"run", "--json", "--ready"}
					if stream {
						argv = append(argv, "--stream")
					}
					argv = append(append(argv, "--"), actionCommand(t, mode)...)
					cmd := exec.Command(binary, argv...)
					cmd.Dir = root
					var out, errout bytes.Buffer
					cmd.Stdout, cmd.Stderr = &out, &errout
					if err := cmd.Start(); err != nil {
						t.Fatal(err)
					}
					finished := make(chan error, 1)
					go func() { finished <- cmd.Wait() }()
					t.Cleanup(func() {
						cmd.Process.Kill()
						for _, name := range []string{"child.pid", "descendant.pid"} {
							if data, err := os.ReadFile(filepath.Join(root, name)); err == nil {
								pid, _ := strconv.Atoi(string(data))
								if pid > 0 {
									syscall.Kill(pid, syscall.SIGKILL)
								}
							}
						}
					})
					deadline := time.Now().Add(8 * time.Second)
					for {
						childPID, childErr := os.ReadFile(filepath.Join(root, "descendant.pid"))
						partial, partialErr := os.ReadFile(filepath.Join(root, "partial-change"))
						// Existence alone races the helper's open/truncate/write.
						if childErr == nil && len(childPID) > 0 && partialErr == nil && string(partial) == "retained" {
							break
						}
						if time.Now().After(deadline) {
							t.Fatal("child did not become ready")
						}
						time.Sleep(10 * time.Millisecond)
					}
					if err := cmd.Process.Signal(sig); err != nil {
						t.Fatal(err)
					}
					select {
					case err := <-finished:
						exit, ok := err.(*exec.ExitError)
						if !ok || exit.ExitCode() != 130 {
							t.Fatalf("exit %v stdout %s stderr %s", err, out.String(), errout.String())
						}
					case <-time.After(8 * time.Second):
						t.Fatal("runner did not finish bounded group shutdown")
					}
					e := runFinalEnvelope(t, out.Bytes())
					var event struct {
						Outcome runner.Result `json:"outcome"`
					}
					if err := json.Unmarshal(e.Result, &event); err != nil {
						t.Fatal(err)
					}
					result := event.Outcome
					if result.Successful != 0 || result.Failure.Kind != "interrupted" || result.LastAction.Signal == "" || !reflect.DeepEqual(actionIDs(t, root), ids[:1]) {
						t.Fatalf("%+v", result)
					}
					if data, err := os.ReadFile(filepath.Join(root, "partial-change")); err != nil || string(data) != "retained" {
						t.Fatal("partial change lost")
					}
					e = run(t, root, "", 0, "show", ids[0], "--json")
					if resultMap := resultTicket(t, e); resultMap["status"] != "in-progress" {
						t.Fatal("partial ticket mutation lost")
					}
					for _, name := range []string{"child.pid", "descendant.pid"} {
						data, _ := os.ReadFile(filepath.Join(root, name))
						pid, _ := strconv.Atoi(string(data))
						// Orphan zombies may await init's reap on Linux; they cannot run.
						deadline := time.Now().Add(3 * time.Second)
						for syscall.Kill(pid, 0) == nil {
							state, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
							if strings.HasPrefix(strings.TrimSpace(string(state)), "Z") {
								break
							}
							if time.Now().After(deadline) {
								t.Fatalf("%s still running: %s", name, state)
							}
							time.Sleep(20 * time.Millisecond)
						}
					}
				})
			}
		}
	}
}

func resultTicket(t *testing.T, e envelope) map[string]any {
	t.Helper()
	return result(t, e)["ticket"].(map[string]any)
}

// Decode each NDJSON line independently, including the final outcome. This also
// rejects raw child bytes, multiple JSON values on a line, and missing newlines.
func runFinalEnvelope(t *testing.T, data []byte) envelope {
	t.Helper()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("unterminated run output: %q", data)
	}
	var last envelope
	lastKind := ""
	for i, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
		if err := json.Unmarshal(line, &last); err != nil {
			t.Fatalf("event: %v: %s", err, line)
		}
		var event struct {
			Event string `json:"event"`
		}
		if err := json.Unmarshal(last.Result, &event); err != nil {
			t.Fatal(err)
		}
		if i == 0 && event.Event != "started" {
			t.Fatalf("first event: %s", line)
		}
		lastKind = event.Event
	}
	if lastKind != "finished" {
		t.Fatalf("final event: %s", data)
	}
	return last
}
