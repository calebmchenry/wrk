//go:build darwin || linux

package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"wrk/internal/runner"
)

func TestRunNoisyChildFullLogs(t *testing.T) {
	root, _ := runProject(t, 1)
	r, stdout, stderr := runAction(t, root, 0, nil, actionCommand(t, "noisy"))
	if stderr != "" || len(stdout) > 60000 || r.Successful != 1 {
		t.Fatalf("output not isolated: stdout %d stderr %d %+v", len(stdout), len(stderr), r)
	}
	a := r.LastAction
	for path, chunk := range map[string]string{a.StdoutLog: "out{}\x00\x1b\n", a.StderrLog: "err{}\x00\x1b\n"} {
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, bytes.Repeat([]byte(chunk), 1024*1024)) {
			t.Fatalf("incomplete %s: %d bytes, %v", path, len(data), err)
		}
	}
	if a.Output.StdoutBytes != 8*1024*1024 || a.Output.StderrBytes != 8*1024*1024 {
		t.Fatalf("activity %+v", a.Output)
	}
}

func TestRunJSONFlushesWhileChildIsAlive(t *testing.T) {
	root, _ := runProject(t, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	argv := append([]string{"run", "--json", "--verbose", "--heartbeat-interval=20ms", "--max-tickets=1", "--"}, actionCommand(t, "gated")...)
	cmd := exec.CommandContext(ctx, binary, argv...)
	cmd.Dir = root
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill() })
	scanner := bufio.NewScanner(stdout)
	var kinds []string
	quiet, noisy, finished := false, false, false
	var previous time.Time
	for scanner.Scan() {
		var e struct {
			Result runner.Event `json:"result"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("corrupt JSON %s: %v", scanner.Bytes(), err)
		}
		event := e.Result
		if event.Time.Before(previous) {
			t.Fatal("timestamps reversed")
		}
		previous = event.Time
		kinds = append(kinds, event.Kind)
		if event.Kind == "heartbeat" {
			if event.Action.Ended != nil || event.Action.Elapsed <= 0 || event.Action.Sequence != 1 {
				t.Fatalf("heartbeat %+v", event)
			}
			if !quiet {
				if event.Action.Output.LastOutput != nil {
					t.Fatalf("invented output: %+v", event.Action.Output)
				}
				quiet = true
				put(t, root, "emit-output", "")
			} else if event.Action.Output.StdoutBytes == 11 && event.Action.Output.StderrBytes == 11 {
				if event.Action.Output.LastOutput == nil || event.Action.Output.LastOutput.After(event.Time) {
					t.Fatal("bad output time")
				}
				noisy = true
				put(t, root, "finish-action", "")
			}
		}
		if event.Kind == "finished" {
			finished = event.Result.Reason == "limit" && event.Result.Successful == 1
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("%v %s", err, stderr.String())
	}
	if !quiet || !noisy || !finished || kinds[0] != "started" || kinds[1] != "selection" || kinds[2] != "action_started" || kinds[len(kinds)-2] != "action_finished" {
		t.Fatalf("events %v; quiet/noisy/finished %t/%t/%t", kinds, quiet, noisy, finished)
	}
	if !strings.Contains(stderr.String(), "live stdout") || !strings.Contains(stderr.String(), "live stderr") {
		t.Fatal(stderr.String())
	}
}

func TestRunRetryCommandRoundTripsArguments(t *testing.T) {
	t.Setenv("WRK_RUN_TEST_HELPER", "1")
	root, ids := runProject(t, 1)
	launch, _ := runProject(t, 0)
	// The selected project itself has whitespace and quotes in its path.
	selected := filepath.Join(launch, "selected project's tickets")
	if err := os.Rename(root, selected); err != nil {
		t.Fatal(err)
	}
	self, _ := os.Executable()
	// First launch uses an executable that fails; retry changes that script to
	// capture exactly what the suggested command passes through the shell.
	script := filepath.Join(selected, "retry-action")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	args := []string{"./retry-action", "", "two words", "a'b", "line\nbreak", "$(touch unwanted);*", "{id}"}
	_, data, _ := runAction(t, launch, 1, []string{"--config", filepath.Join(selected, ".wrk/config.yaml"), "--expect-status=todo"}, args)
	e := runFinalEnvelope(t, []byte(data))
	var event struct {
		Retry string `json:"retry_command"`
		Show  string `json:"show_command"`
	}
	if err := json.Unmarshal(e.Result, &event); err != nil {
		t.Fatal(err)
	}
	if event.Retry == "" || event.Show == "" {
		t.Fatal(string(e.Result))
	}
	// A small wrapper passes through the retry arguments to the existing helper.
	wrapper := "#!/bin/sh\nexec '" + strings.ReplaceAll(self, "'", "'\"'\"'") + "' -test.run=^TestRunChild$ -- capture '" + ids[0] + "' \"$@\"\n"
	if err := os.WriteFile(script, []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(binary)+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd := exec.Command("/bin/sh", "-c", event.Retry)
	cmd.Dir = launch
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("retry: %v %s", err, data)
	}
	var captured struct{ Args []string }
	content, err := os.ReadFile(filepath.Join(selected, "capture.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &captured); err != nil {
		t.Fatal(err)
	}
	want := append([]string{}, args[1:]...)
	want[len(want)-1] = ids[0]
	if strings.Join(captured.Args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("got %q want %q", captured.Args, want)
	}
	if _, err := os.Stat(filepath.Join(selected, "unwanted")); !os.IsNotExist(err) {
		t.Fatal("shell metacharacters evaluated")
	}
}

func TestRunClosedJSONPipeStopsProcessGroup(t *testing.T) {
	root, ids := runProject(t, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	argv := append([]string{"run", "--json", "--heartbeat-interval=20ms", "--"}, actionCommand(t, "interrupt")...)
	cmd := exec.CommandContext(ctx, binary, argv...)
	cmd.Dir = root
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
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
	scanner := bufio.NewScanner(stdout)
	var logPath string
	for scanner.Scan() {
		var e struct {
			Result struct {
				Event  string `json:"event"`
				RunLog string `json:"run_log"`
			} `json:"result"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		logPath = e.Result.RunLog
		partialBytes, partial := os.ReadFile(filepath.Join(root, "partial-change"))
		descendantPID, descendant := os.ReadFile(filepath.Join(root, "descendant.pid"))
		if e.Result.Event == "heartbeat" && partial == nil && string(partialBytes) == "retained" && descendant == nil && len(descendantPID) > 0 {
			break
		}
	}
	stdout.Close() // Actual EPIPE/SIGPIPE on fd 1, not an injected Go writer.
	err = cmd.Wait()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 || !strings.Contains(stderr.String(), "RUN_OUTPUT") {
		t.Fatalf("exit %v; %s", err, stderr.String())
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	e := runFinalEnvelope(t, data)
	var final struct {
		Outcome runner.Result `json:"outcome"`
	}
	if err := json.Unmarshal(e.Result, &final); err != nil {
		t.Fatal(err)
	}
	if final.Outcome.Reason != "output" || final.Outcome.Successful != 0 || len(actionIDs(t, root)) != 1 || actionIDs(t, root)[0] != ids[0] {
		t.Fatalf("%+v", final)
	}
	for _, name := range []string{"child.pid", "descendant.pid"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		pid, _ := strconv.Atoi(string(data))
		deadline := time.Now().Add(2 * time.Second)
		for syscall.Kill(pid, 0) == nil {
			state, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
			if strings.HasPrefix(strings.TrimSpace(string(state)), "Z") {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s survived broken pipe", name)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
