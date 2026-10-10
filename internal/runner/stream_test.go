//go:build darwin || linux

package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"testing/synctest"
	"time"
	"wrk/internal/store"
	"wrk/internal/ticket"
)

func streamOptions(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	if m := store.Init(root); len(m.Diagnostics) != 0 {
		t.Fatal(m.Diagnostics)
	}
	return Options{Root: root, Stream: true, PollInterval: DefaultPollInterval,
		Command: []string{"/bin/sh", "-c", "exit 0"}, Stdout: io.Discard, Stderr: io.Discard}
}

func streamTicket(t *testing.T, o Options, parent *string, labels ...string) string {
	t.Helper()
	m := store.Create(o.Root, store.CreateOptions{Title: "Stream action", Parent: parent, Labels: labels, LabelsSet: true})
	if len(m.Diagnostics) != 0 {
		t.Fatal(m.Diagnostics)
	}
	return m.Ticket.ID
}

func streamUpdate(t *testing.T, o Options, id string, c ticket.Changes) {
	t.Helper()
	if m := store.UpdateWithOptions(o.Root, id, store.UpdateOptions{Changes: c}); len(m.Diagnostics) != 0 {
		t.Fatal(m.Diagnostics)
	}
}

func streamStatus(t *testing.T, o Options, id, status string) {
	t.Helper()
	streamUpdate(t, o, id, ticket.Changes{Status: &status})
}

func TestStreamEligibilityOnLaterPoll(t *testing.T) {
	for _, mode := range []string{"new", "dependency", "status", "label", "parent"} {
		t.Run(mode, func(t *testing.T) {
			o := streamOptions(t)
			parent := streamTicket(t, o, nil, "batch", "second")
			o.Query = Query{Ready: true, Labels: []string{"batch", "second"}, Under: &parent}
			o.MaxTickets = 1
			var id, dependency string
			switch mode {
			case "dependency":
				dependency = streamTicket(t, o, nil)
				id = streamTicket(t, o, &parent, "batch", "second")
				streamUpdate(t, o, id, ticket.Changes{AddDependencies: []string{dependency}})
			case "status":
				id = streamTicket(t, o, &parent, "batch", "second")
				streamStatus(t, o, id, "blocked")
			case "label":
				id = streamTicket(t, o, &parent, "batch")
			case "parent":
				id = streamTicket(t, o, nil, "batch", "second")
			}
			var events []string
			var started time.Time
			o.OnEvent = func(e Event) error {
				events = append(events, e.Kind)
				if !e.Stream || e.PollInterval != DefaultPollInterval || !reflect.DeepEqual(e.Query, o.Query) {
					t.Fatalf("lost configuration: %+v", e)
				}
				if e.Kind == "idle_started" {
					if e.Matches != 0 || e.Processed != 0 || e.Unprocessed != 0 || e.Idle == nil {
						t.Fatalf("initial idle: %+v", e)
					}
					started = e.Idle.Started
				}
				if e.Kind == "idle_finished" && (e.Idle.Started != started || e.Idle.Elapsed <= 0 || e.Unprocessed != 1) {
					t.Fatalf("idle finish: %+v", e)
				}
				if e.Kind == "action_started" && e.Action.Ticket.ID != id {
					t.Fatalf("unexpected selection: %+v", e.Action)
				}
				return nil
			}
			polls := 0
			r := run(context.Background(), o, func(ctx context.Context, interval time.Duration) {
				polls++
				if interval != DefaultPollInterval || polls > 2 {
					t.Fatalf("unexpected wait %d: %s", polls, interval)
				}
				if polls == 1 {
					return // Unchanged poll must be silent.
				}
				switch mode {
				case "new":
					id = streamTicket(t, o, &parent, "batch", "second")
				case "dependency":
					streamStatus(t, o, dependency, "done")
				case "status":
					streamStatus(t, o, id, "todo")
				case "label":
					streamUpdate(t, o, id, ticket.Changes{AddLabels: []string{"second"}})
				case "parent":
					streamUpdate(t, o, id, ticket.Changes{Parent: &parent})
				}
			})
			want := []string{"started", "idle_started", "idle_finished", "selection", "action_started", "action_finished", "finished"}
			if r.ExitCode() != 0 || r.Reason != "limit" || r.Successful != 1 || polls != 2 || !reflect.DeepEqual(events, want) {
				t.Fatalf("result %+v polls %d events %v", r, polls, events)
			}
		})
	}
}

func TestStreamProcessedIDsSurvivePollsAndReentry(t *testing.T) {
	o := streamOptions(t)
	o.Query = Query{Ready: true, Labels: []string{"batch"}}
	o.MaxTickets = 4
	ids := []string{streamTicket(t, o, nil, "batch"), streamTicket(t, o, nil, "batch")}
	slices.Sort(ids)
	polls, idleStarts, idleChanges := 0, 0, 0
	var actions []string
	o.OnEvent = func(e Event) error {
		switch e.Kind {
		case "action_started":
			if len(actions) < 2 && polls != 0 {
				t.Fatal("waited before immediately available action")
			}
			actions = append(actions, e.Action.Ticket.ID)
		case "idle_started":
			idleStarts++
			if e.Matches != len(actions) || e.Processed != len(actions) || e.Unprocessed != 0 || e.Successful != len(actions) {
				t.Fatalf("all-processed idle: %+v", e)
			}
		case "idle_changed":
			idleChanges++
			if e.Matches != e.Processed || e.Unprocessed != 0 || e.Successful != 2 {
				t.Fatalf("reentered ID was not retained: %+v", e)
			}
		}
		return nil
	}
	r := run(context.Background(), o, func(context.Context, time.Duration) {
		polls++
		switch polls {
		case 1, 2: // Repeated all-processed queries are silent and do not run again.
		case 3:
			streamStatus(t, o, ids[0], "done")
		case 4:
			streamStatus(t, o, ids[0], "todo")
		case 5:
			streamUpdate(t, o, ids[1], ticket.Changes{RemoveLabels: []string{"batch"}})
		case 6:
			streamUpdate(t, o, ids[1], ticket.Changes{AddLabels: []string{"batch"}})
		case 7, 8:
			ids = append(ids, streamTicket(t, o, nil, "batch"))
		default:
			t.Fatal("limit did not stop the stream")
		}
	})
	if r.ExitCode() != 0 || r.Reason != "limit" || r.Successful != 4 || polls != 8 || idleStarts != 2 || idleChanges != 4 || !reflect.DeepEqual(actions, ids) {
		t.Fatalf("%+v polls %d idle %d/%d actions %v want %v", r, polls, idleStarts, idleChanges, actions, ids)
	}
	// A fresh invocation has no memory, even in stream mode.
	o.OnEvent = nil
	r = run(context.Background(), o, func(context.Context, time.Duration) { t.Fatal("fresh invocation went idle") })
	if r.Successful != 4 || r.Reason != "limit" {
		t.Fatalf("fresh invocation: %+v", r)
	}
}

func TestStreamScopeChangesAndIdleInterruption(t *testing.T) {
	o := streamOptions(t)
	parent := streamTicket(t, o, nil, "batch", "second")
	outside := streamTicket(t, o, nil, "batch", "second")
	streamTicket(t, o, &parent, "batch") // Missing the second AND label.
	o.Query = Query{Ready: true, Labels: []string{"batch", "second"}, Under: &parent}
	var active string
	for _, status := range []string{"todo", "in-progress", "blocked", "done", "canceled"} {
		id := streamTicket(t, o, &parent, "batch", "second")
		streamStatus(t, o, id, status)
		if status == "todo" {
			streamUpdate(t, o, id, ticket.Changes{AddDependencies: []string{outside}})
		}
		if status == "in-progress" {
			active = id
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var events []string
	var lastIdle time.Duration
	polls := 0
	o.OnEvent = func(e Event) error {
		events = append(events, e.Kind)
		if e.Matches != 0 || e.Unprocessed != 0 {
			t.Fatalf("diagnostics expanded eligibility: %+v", e)
		}
		if e.Idle != nil {
			if e.Idle.Elapsed < lastIdle {
				t.Fatal("idle duration reset")
			}
			lastIdle = e.Idle.Elapsed
		}
		if e.Kind == "idle_started" && e.Scope != (ScopeCounts{Waiting: 1, InProgress: 1, Blocked: 1, Done: 1, Canceled: 1}) {
			t.Fatalf("initial scope: %+v", e.Scope)
		}
		if e.Kind == "idle_changed" && e.Scope != (ScopeCounts{Waiting: 1, Blocked: 2, Done: 1, Canceled: 1}) {
			t.Fatalf("changed scope: %+v", e.Scope)
		}
		return nil
	}
	r := run(ctx, o, func(context.Context, time.Duration) {
		polls++
		if polls == 1 {
			streamStatus(t, o, active, "blocked")
		} else {
			cancel()
		}
	})
	want := []string{"started", "idle_started", "idle_changed", "idle_finished", "finished"}
	if r.ExitCode() != 130 || r.Successful != 0 || r.LastAction != nil || polls != 2 || !reflect.DeepEqual(events, want) {
		t.Fatalf("%+v polls %d events %v", r, polls, events)
	}
}

func TestStreamParentRenameChangesExplanation(t *testing.T) {
	o := streamOptions(t)
	parent := streamTicket(t, o, nil)
	o.Query.Under = &parent
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	polls, changes := 0, 0
	o.OnEvent = func(e Event) error {
		if e.Kind == "idle_changed" {
			changes++
			if e.Parent.Title != "Renamed parent" || e.Matches != 0 || e.Scope != (ScopeCounts{}) {
				t.Fatalf("%+v", e)
			}
		}
		return nil
	}
	r := run(ctx, o, func(context.Context, time.Duration) {
		polls++
		if polls == 1 {
			title := "Renamed parent"
			streamUpdate(t, o, parent, ticket.Changes{Title: &title})
		} else {
			cancel()
		}
	})
	if r.ExitCode() != 130 || changes != 1 {
		t.Fatalf("%+v changes %d", r, changes)
	}
}

func TestStreamLaterFailuresStop(t *testing.T) {
	for _, mode := range []string{"exit", "completion", "project", "selection", "idle_started", "idle_changed", "idle_finished", "child_output"} {
		t.Run(mode, func(t *testing.T) {
			o := streamOptions(t)
			o.Query.Ready = true
			expected := mode
			var parent string
			switch mode {
			case "exit":
				o.Command[2] = "exit 7"
			case "completion":
				o.ExpectStatus = "done"
			case "selection":
				parent = streamTicket(t, o, nil)
				o.Query.Under = &parent
			case "idle_started", "idle_changed", "idle_finished":
				expected = "output"
			case "child_output":
				expected = "output"
				o.Command[2] = "printf output"
				o.Stdout = brokenStreamOutput{}
			}
			starts, polls := 0, 0
			o.OnEvent = func(e Event) error {
				if e.Kind == "action_started" {
					starts++
				}
				if e.Kind == mode {
					return errors.New("cannot record idle event")
				}
				return nil
			}
			r := run(context.Background(), o, func(context.Context, time.Duration) {
				polls++
				if polls > 1 {
					t.Fatal("waited/retried after failure")
				}
				switch mode {
				case "project":
					if err := os.WriteFile(filepath.Join(o.Root, ".wrk/config.yaml"), []byte("invalid: ["), 0600); err != nil {
						t.Fatal(err)
					}
				case "selection":
					if err := os.Remove(filepath.Join(o.Root, ".wrk", parent+".md")); err != nil {
						t.Fatal(err)
					}
				case "idle_changed":
					streamStatus(t, o, streamTicket(t, o, nil), "blocked")
				default:
					streamTicket(t, o, nil)
					streamTicket(t, o, nil)
				}
			})
			wantStarts := 0
			if mode == "exit" || mode == "completion" || mode == "child_output" {
				wantStarts = 1
			}
			if r.ExitCode() != 1 || r.Failure.Kind != expected || r.Successful != 0 || starts != wantStarts {
				t.Fatalf("%+v starts %d", r, starts)
			}
		})
	}
}

type brokenStreamOutput struct{}

func (brokenStreamOutput) Write([]byte) (int, error) { return 0, errors.New("broken output") }

func TestIdleTimer(t *testing.T) {
	for _, interrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "interval", true: "interrupt"}[interrupt], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				finished := make(chan struct{})
				start := time.Now()
				go func() {
					waitIdle(ctx, time.Hour)
					close(finished)
				}()
				synctest.Wait()
				select {
				case <-finished:
					t.Fatal("timer did not wait")
				default:
				}
				if interrupt {
					cancel()
				} else {
					time.Sleep(time.Hour)
				}
				synctest.Wait()
				want := time.Duration(0)
				if !interrupt {
					want = time.Hour
				}
				select {
				case <-finished:
				default:
					t.Fatal("timer did not finish")
				}
				if time.Since(start) != want {
					t.Fatalf("finished after %s, want %s", time.Since(start), want)
				}
			})
		})
	}
}
