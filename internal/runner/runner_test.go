//go:build darwin || linux

package runner

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"
	"wrk/internal/store"
)

func testOptions(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	if m := store.Init(root); len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	for range 2 {
		if m := store.Create(root, store.CreateOptions{Title: "Action"}); len(m.Diagnostics) > 0 {
			t.Fatal(m.Diagnostics)
		}
	}
	return Options{Root: root, Command: []string{"/bin/sh", "-c", "exit 0"}, Stdout: io.Discard, Stderr: io.Discard}
}

func TestLifecycleFacts(t *testing.T) {
	o := testOptions(t)
	var kinds []string
	var lastID string
	queries := 0
	o.OnEvent = func(e Event) error {
		kinds = append(kinds, e.Kind)
		if e.Time.IsZero() || e.Root != o.Root || !reflect.DeepEqual(e.Command, o.Command) {
			t.Fatalf("missing context: %+v", e)
		}
		switch e.Kind {
		case "selection":
			if e.Matches != 2 || e.Unprocessed != 2-queries || e.Successful != queries {
				t.Fatalf("query: %+v", e)
			}
			queries++
		case "action_started":
			if e.Action.Ticket.ID <= lastID || e.Action.ExitCode != nil {
				t.Fatalf("start: %+v", e.Action)
			}
			lastID = e.Action.Ticket.ID
		case "action_finished":
			a := e.Action
			if a.Ticket.ID != lastID || a.ExitCode == nil || *a.ExitCode != 0 || a.ObservedStatus != "todo" || a.Elapsed <= 0 || a.Failure != nil || e.Successful != queries {
				t.Fatalf("finish: %+v; count %d", a, e.Successful)
			}
		case "finished":
			if e.Result.Successful != 2 || e.Result.Reason != "drained" || e.Result.Elapsed <= 0 {
				t.Fatalf("result: %+v", e.Result)
			}
		}
		return nil
	}
	result := Run(context.Background(), o)
	want := []string{"started", "selection", "action_started", "action_finished", "selection", "action_started", "action_finished", "selection", "finished"}
	if result.ExitCode() != 0 || !reflect.DeepEqual(kinds, want) {
		t.Fatalf("%+v events %v", result, kinds)
	}
}

func TestFailureFacts(t *testing.T) {
	for _, mode := range []string{"exit", "signal", "completion", "start"} {
		t.Run(mode, func(t *testing.T) {
			o := testOptions(t)
			switch mode {
			case "exit":
				o.Command[2] = "exit 17"
			case "signal":
				o.Command[2] = "kill -TERM $$"
			case "completion":
				o.ExpectStatus = "done"
			case "start":
				o.Command[0] = "./no-such-executable"
			}
			starts, finishes := 0, 0
			o.OnEvent = func(e Event) error {
				if e.Kind == "action_started" {
					starts++
				}
				if e.Kind == "action_finished" {
					finishes++
					if e.Action.Failure == nil || e.Successful != 0 {
						t.Fatalf("%+v", e)
					}
				}
				return nil
			}
			r := Run(context.Background(), o)
			if r.ExitCode() != 1 || r.Successful != 0 || starts != 1 || finishes != 1 {
				t.Fatalf("%+v", r)
			}
			a := r.LastAction
			switch mode {
			case "exit":
				if *a.ExitCode != 17 || a.Failure.Kind != "exit" {
					t.Fatalf("%+v", a)
				}
			case "signal":
				if *a.ExitCode != -1 || a.Signal == "" || a.Failure.Kind != "exit" {
					t.Fatalf("%+v", a)
				}
			case "completion":
				if *a.ExitCode != 0 || a.ObservedStatus != "todo" || a.Failure.Kind != "completion" {
					t.Fatalf("%+v", a)
				}
			case "start":
				if a.ExitCode != nil || a.Failure.Kind != "start" {
					t.Fatalf("%+v", a)
				}
			}
		})
	}
}

func TestCancellationAndOutputFailureBetweenActions(t *testing.T) {
	for _, event := range []string{"selection", "action_started", "action_finished"} {
		for _, cancelRun := range []bool{false, true} {
			t.Run(event+map[bool]string{false: "/output", true: "/cancel"}[cancelRun], func(t *testing.T) {
				o := testOptions(t)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				starts := 0
				o.OnEvent = func(e Event) error {
					if e.Kind == "action_started" {
						starts++
					}
					if e.Kind == event {
						if cancelRun {
							cancel()
						} else {
							return errors.New("broken output")
						}
					}
					return nil
				}
				r := Run(ctx, o)
				wantCount := 0
				if event == "action_finished" {
					wantCount = 1
				}
				wantCode := 1
				if cancelRun {
					wantCode = 130
				}
				if r.ExitCode() != wantCode || r.Successful != wantCount || starts > 1 {
					t.Fatalf("%+v starts %d", r, starts)
				}
			})
		}
	}
}

func TestInheritedOutputPipeHasBoundedWait(t *testing.T) {
	o := testOptions(t)
	// io.Discard causes os/exec to copy a pipe; the descendant holds it open
	// after its leader exits. A timeout must fail instead of accepting exit 0.
	o.Command[2] = "sleep 60 & exit 0"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	r := Run(ctx, o)
	if r.ExitCode() != 1 || r.Failure.Kind != "wait" || r.Successful != 0 || time.Since(start) > 8*time.Second {
		t.Fatalf("%+v", r)
	}
}
