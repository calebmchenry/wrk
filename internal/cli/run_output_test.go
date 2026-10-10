package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"wrk/internal/project"
	"wrk/internal/runner"
	"wrk/internal/store"
)

func TestRunOutputFlags(t *testing.T) {
	for _, args := range [][]string{
		{"run", "--heartbeat-interval=0s", "--", "true"},
		{"run", "--heartbeat-interval=-1s", "--", "true"},
		{"run", "--heartbeat-interval=bad", "--", "true"},
		{"run", "--log-dir=", "--", "true"},
		{"run", "--verbose=false", "--", "true"},
		{"run", "--heartbeat-interval=1s", "--heartbeat-interval=2s", "--", "true"},
		{"list", "--verbose"}, {"list", "--log-dir=logs"},
	} {
		if _, err := Parse(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	r, err := Parse([]string{"run", "--verbose", "--json", "--log-dir=other logs", "--heartbeat-interval=25ms", "--", "true"})
	if err != nil || !r.Verbose || !r.JSON || r.LogDir != "other logs" || r.HeartbeatInterval != 25*time.Millisecond {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = Parse([]string{"run", "--", "true"})
	if err != nil || r.HeartbeatInterval != 60*time.Second {
		t.Fatalf("%+v %v", r, err)
	}
	for _, flag := range []string{"--verbose", "--log-dir", "--heartbeat-interval"} {
		if !strings.Contains(Usage, flag) {
			t.Errorf("help missing %s", flag)
		}
	}
}

func TestSelectionDescription(t *testing.T) {
	parent, id := "wrk-12345678", "wrk-abcdef12"
	e := runner.Event{Query: runner.Query{Ready: true, Labels: []string{"first", "two words"}, Under: &parent}, Parent: &project.Summary{ID: parent, Title: "Parent title"}}
	want := `ready: todo with all dependencies done AND all labels: "first", "two words" AND descendants of "Parent title" (wrk-12345678) (parent excluded)`
	if got := describeSelection(e); got != want {
		t.Fatalf("%q want %q", got, want)
	}
	e.Query = runner.Query{}
	if got := describeSelection(e); got != "active statuses: todo, in-progress, blocked" {
		t.Fatal(got)
	}
	e.Query.All = true
	if got := describeSelection(e); got != "all statuses" {
		t.Fatal(got)
	}
	e.Query = runner.Query{Ticket: &id}
	if got := describeSelection(e); got != "explicit ticket wrk-abcdef12 (readiness not required)" {
		t.Fatal(got)
	}
}

func outputProject(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if m := store.Init(root); len(m.Diagnostics) != 0 {
		t.Fatal(m.Diagnostics)
	}
	m := store.Create(root, store.CreateOptions{Title: "Observed action"})
	if len(m.Diagnostics) != 0 {
		t.Fatal(m.Diagnostics)
	}
	return root, m.Ticket.ID
}

func TestRunRecordsAndChildIsolation(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		t.Run(map[bool]string{false: "quiet", true: "verbose"}[verbose], func(t *testing.T) {
			root, id := outputProject(t)
			cwd := t.TempDir()
			var out, errout bytes.Buffer
			r, err := Parse([]string{"run", "--json", "--log-dir=relative logs", "--", "/bin/sh", "-c", "printf 'stdout bytes'; printf 'stderr bytes' >&2"})
			if err != nil {
				t.Fatal(err)
			}
			r.Verbose = verbose
			if code := runActions(r, root, cwd, &out, &errout); code != 0 {
				t.Fatalf("%d %s %s", code, out.String(), errout.String())
			}
			if verbose && errout.Len() != len("stdout bytesstderr bytes") || !verbose && errout.Len() != 0 {
				t.Fatalf("stderr: %q", errout.String())
			}
			var events []runRecord
			for _, line := range bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n")) {
				var e struct {
					Schema int       `json:"schema_version"`
					OK     bool      `json:"ok"`
					Root   string    `json:"project_root"`
					Result runRecord `json:"result"`
				}
				if err := json.Unmarshal(line, &e); err != nil {
					t.Fatalf("%v %s", err, line)
				}
				if e.Schema != 1 || !e.OK || e.Root != root {
					t.Fatalf("%+v", e)
				}
				events = append(events, e.Result)
			}
			var kinds []string
			for _, e := range events {
				kinds = append(kinds, e.Kind)
			}
			if !reflect.DeepEqual(kinds, []string{"started", "selection", "action_started", "action_finished", "selection", "finished"}) {
				t.Fatal(kinds)
			}
			last := events[len(events)-1]
			if !strings.HasPrefix(last.RunDirectory, filepath.Join(cwd, "relative logs")+string(os.PathSeparator)) {
				t.Fatal(last.RunDirectory)
			}
			if last.Result.Successful != 1 || last.Matches != 1 || last.Processed != 1 || last.Unprocessed != 0 {
				t.Fatalf("%+v", last)
			}
			a := last.Result.LastAction
			if a.Ticket.ID != id || a.Sequence != 1 || a.Started.IsZero() || a.Ended == nil || a.CompletionCheck != "passed" || a.ObservedStatus != "todo" || a.Output.LastOutput == nil || a.Output.StdoutBytes != 12 || a.Output.StderrBytes != 12 {
				t.Fatalf("%+v", a)
			}
			for file, want := range map[string]string{a.StdoutLog: "stdout bytes", a.StderrLog: "stderr bytes", last.RunLog: out.String()} {
				data, err := os.ReadFile(file)
				if err != nil || string(data) != want {
					t.Fatalf("%s: %v %q", file, err, data)
				}
				info, err := os.Stat(file)
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatalf("log permissions: %v %v", info, err)
				}
			}
		})
	}
}

func TestHumanRunningAndIdleOutput(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(map[bool]string{false: "redirected", true: "terminal"}[terminal], func(t *testing.T) {
			var out, errout bytes.Buffer
			w := &runOutput{out: &out, errout: &errout, terminal: terminal}
			now := time.Now()
			e := runner.Event{Kind: "heartbeat", Time: now, Action: &runner.Action{Sequence: 1, Ticket: project.Summary{ID: "wrk-12345678"}, Elapsed: time.Minute + 2*time.Second}}
			if err := w.human(runRecord{Event: e}, nil); err != nil {
				t.Fatal(err)
			}
			e.Action.Output.LastOutput = &now
			e.Time = now.Add(15 * time.Second)
			if err := w.human(runRecord{Event: e}, nil); err != nil {
				t.Fatal(err)
			}
			e.Kind = "action_finished"
			if err := w.human(runRecord{Event: e}, nil); err != nil {
				t.Fatal(err)
			}
			text := errout.String()
			if !strings.Contains(text, "elapsed 1m2s; no child output observed") || !strings.Contains(text, "last child output 15s ago") || !strings.Contains(text, "Finished wrk-12345678") {
				t.Fatal(text)
			}
			if strings.Contains(text, "\x1b[2K") != terminal || strings.Contains(text, "\r") != terminal {
				t.Fatalf("wrong controls %q", text)
			}
			for _, invented := range []string{"ETA", "%", "hung", "testing", "implementing"} {
				if strings.Contains(text, invented) {
					t.Fatal(text)
				}
			}
		})
	}
	var out, errout bytes.Buffer
	w := &runOutput{out: &out, errout: &errout}
	e := runner.Event{Kind: "idle_started", PollInterval: 5 * time.Second, Idle: &runner.Idle{}, SelectionCounts: runner.SelectionCounts{Matches: 3, Processed: 3, Scope: runner.ScopeCounts{Ready: 1, Waiting: 2, InProgress: 3, Blocked: 4, Done: 5, Canceled: 6}}}
	if err := w.human(runRecord{Event: e}, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"all current matches already processed", "do not imply tickets are done", "1 ready, 2 todo waiting on dependencies, 3 in-progress, 4 manually blocked, 5 done, 6 canceled", "diagnostics only"} {
		if !strings.Contains(errout.String(), want) {
			t.Fatalf("missing %s: %s", want, errout.String())
		}
	}
}

type failedRunWriter struct{}

func (failedRunWriter) Write([]byte) (int, error) { return 0, errors.New("destination failed") }

func TestLogAndEventFailuresStopExecution(t *testing.T) {
	for _, mode := range []string{"run log", "child log", "heartbeat output", "live output", "close child", "close run", "create child"} {
		t.Run(mode, func(t *testing.T) {
			root, _ := outputProject(t)
			if m := store.Create(root, store.CreateOptions{Title: "Must not run next"}); len(m.Diagnostics) != 0 {
				t.Fatal(m.Diagnostics)
			}
			var out, errout bytes.Buffer
			r := Request{JSON: true, Verbose: mode == "live output"}
			w, err := newRunOutput(r, root, root, &out, &errout)
			if err != nil {
				t.Fatal(err)
			}
			defer w.close()
			starts := 0
			o := runner.Options{Root: root, Command: []string{"/bin/sh", "-c", "printf bytes; exec sleep 30"}, HeartbeatInterval: 20 * time.Millisecond, Stdout: runChildWriter{recorder: w}, Stderr: runChildWriter{recorder: w, stderr: true}}
			if mode == "close child" || mode == "close run" {
				o.Command[2] = "exit 0"
				o.MaxTickets = 1
			}
			o.OnEvent = func(e runner.Event) error {
				if e.Kind == "action_started" {
					starts++
					if mode == "create child" {
						if err := os.RemoveAll(w.dir); err != nil {
							t.Fatal(err)
						}
					}
				}
				if e.Kind == "action_finished" && mode == "close child" {
					w.stdout.Close()
				}
				if e.Kind == "finished" && mode == "close run" {
					w.log.Close()
				}
				err := w.event(e)
				if e.Kind == "action_started" {
					switch mode {
					case "run log":
						w.log.Close()
					case "child log":
						w.stdout.Close()
					case "heartbeat output":
						w.out = failedRunWriter{}
					case "live output":
						w.errout = failedRunWriter{}
					}
				}
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			start := time.Now()
			result := runner.Run(ctx, o)
			if result.ExitCode() != 1 || result.Failure.Kind != "output" || starts != 1 || time.Since(start) > 5*time.Second {
				t.Fatalf("%+v starts %d", result, starts)
			}
		})
	}
}

func TestRunSetupFailureAndShortWrite(t *testing.T) {
	root, _ := outputProject(t)
	var out, errout bytes.Buffer
	r := Request{JSON: true, LogDir: filepath.Join(root, ".wrk", "config.yaml"), ChildCommand: []string{"false"}}
	if code := runActions(r, root, root, &out, &errout); code != 1 {
		t.Fatal(code)
	}
	var e Envelope
	if err := json.Unmarshal(out.Bytes(), &e); err != nil || e.OK || e.Result != nil || len(e.Errors) != 1 || e.Errors[0].Code != "RUN_OUTPUT" {
		t.Fatalf("%v %s", err, out.String())
	}
	if err := writeRun(shortRunWriter{}, []byte("event")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
}

type shortRunWriter struct{}

func (shortRunWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
