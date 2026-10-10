// Package runner executes sequential ticket actions without holding a writer lock.
// Selection, execution, and completion facts are independent of CLI rendering.
package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
	"wrk/internal/diagnostic"
	"wrk/internal/project"
	"wrk/internal/ticket"
)

type Query struct {
	All    bool     `json:"all"`
	Ready  bool     `json:"ready"`
	Labels []string `json:"labels"`
	Under  *string  `json:"under"`
	Ticket *string  `json:"ticket"`
}

// Options must come from a validated request, with Root already resolved. Root
// stays fixed: a child changing config, cwd, or query membership cannot redirect
// subsequent queries or completion checks to another project.
type Options struct {
	Root              string
	Query             Query
	Command           []string
	ExpectStatus      string
	MaxTickets        int // zero means unlimited
	Stream            bool
	PollInterval      time.Duration // positive when Stream is true
	HeartbeatInterval time.Duration // zero uses DefaultHeartbeatInterval
	Stdout            io.Writer
	Stderr            io.Writer
	OnEvent           func(Event) error
}

type Failure struct {
	Kind        string                  `json:"kind"`
	Message     string                  `json:"message"`
	Diagnostics []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}

type Action struct {
	Sequence        int             `json:"sequence"`
	Started         time.Time       `json:"started"`
	Ended           *time.Time      `json:"ended,omitempty"`
	RequiredStatus  string          `json:"required_status,omitempty"`
	CompletionCheck string          `json:"completion_check"`
	Output          Activity        `json:"output"`
	StdoutLog       string          `json:"stdout_log,omitempty"`
	StderrLog       string          `json:"stderr_log,omitempty"`
	Ticket          project.Summary `json:"ticket"`
	Command         []string        `json:"command"`
	Elapsed         time.Duration   `json:"elapsed_ns"`
	ExitCode        *int            `json:"exit_code"` // nil if not started; -1 if signaled
	Signal          string          `json:"signal,omitempty"`
	ObservedStatus  string          `json:"observed_status,omitempty"`
	Failure         *Failure        `json:"failure,omitempty"`
}

const DefaultPollInterval = 5 * time.Second
const DefaultHeartbeatInterval = 60 * time.Second

// Activity reports bytes observed by the runner, never semantic progress.
type Activity struct {
	StdoutBytes int64      `json:"stdout_bytes"`
	StderrBytes int64      `json:"stderr_bytes"`
	LastOutput  *time.Time `json:"last_output,omitempty"`
}

// ScopeCounts partitions the same snapshot's label/descendant scope, ignoring
// status filters for diagnostics only. Each ticket contributes exactly once.
type ScopeCounts struct {
	Ready      int `json:"ready"`
	Waiting    int `json:"waiting_on_dependencies"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Done       int `json:"done"`
	Canceled   int `json:"canceled"`
}

type SelectionCounts struct {
	Matches     int         `json:"matches"`
	Processed   int         `json:"processed"`
	Unprocessed int         `json:"unprocessed"`
	Scope       ScopeCounts `json:"scope"`
}

type Idle struct {
	Started time.Time     `json:"started"`
	Elapsed time.Duration `json:"elapsed_ns"`
}

// Events describe selections, idle transitions, actions, and the final outcome. A
// renderer can add logs/JSON without owning a second selection or execution loop.
// Callbacks run synchronously; their errors stop work before another action.
// Unchanged idle polls emit nothing. idle_finished also fires on a handled stop;
// finished carries the stopping reason. Counts always refer to the last valid query.
type Event struct {
	SelectionCounts
	CountsValid       bool             `json:"counts_valid"`
	Kind              string           `json:"event"`
	Time              time.Time        `json:"timestamp"`
	Started           time.Time        `json:"run_started"`
	Elapsed           time.Duration    `json:"elapsed_ns"`
	Root              string           `json:"-"`
	Query             Query            `json:"query"`
	Parent            *project.Summary `json:"parent,omitempty"`
	Command           []string         `json:"command_template"`
	ExpectStatus      string           `json:"required_status,omitempty"`
	MaxTickets        int              `json:"max_tickets"`
	Stream            bool             `json:"stream"`
	PollInterval      time.Duration    `json:"poll_interval_ns"`
	HeartbeatInterval time.Duration    `json:"heartbeat_interval_ns"`
	Idle              *Idle            `json:"idle,omitempty"`
	Successful        int              `json:"successful"`
	Action            *Action          `json:"action,omitempty"`
	Result            *Result          `json:"outcome,omitempty"`
}

type Result struct {
	Successful int           `json:"successful"`
	Reason     string        `json:"reason"`
	Elapsed    time.Duration `json:"elapsed_ns"`
	LastAction *Action       `json:"last_action"`
	Failure    *Failure      `json:"failure,omitempty"`
}

func (r Result) ExitCode() int {
	if r.Failure == nil {
		return 0
	}
	if r.Failure.Kind == "interrupted" {
		return 130
	}
	return 1
}

func Run(ctx context.Context, o Options) (result Result) {
	return run(ctx, o, waitIdle)
}

// The wait boundary lets tests change project state at exact poll boundaries
// without sleeps. Production always uses an interruptible timer.
func run(ctx context.Context, o Options, wait func(context.Context, time.Duration)) (result Result) {
	started := time.Now()
	if o.HeartbeatInterval <= 0 {
		o.HeartbeatInterval = DefaultHeartbeatInterval
	}
	var parent *project.Summary
	announced := false
	processed := map[string]bool{}
	var counts SelectionCounts
	countsValid := false
	var idleStarted time.Time
	emit := func(e Event) error {
		if o.OnEvent == nil {
			return nil
		}
		e.Time, e.Root, e.Query = time.Now(), o.Root, o.Query
		e.Started, e.Elapsed, e.Parent = started, time.Since(started), parent
		e.HeartbeatInterval = o.HeartbeatInterval
		e.Command, e.ExpectStatus, e.MaxTickets = o.Command, o.ExpectStatus, o.MaxTickets
		e.Stream, e.PollInterval = o.Stream, o.PollInterval
		e.SelectionCounts, e.CountsValid = counts, countsValid
		if !idleStarted.IsZero() {
			e.Idle = &Idle{Started: idleStarted, Elapsed: time.Since(idleStarted)}
		}
		e.Successful = result.Successful
		return o.OnEvent(e)
	}
	outputFailure := func(err error) *Failure {
		return &Failure{Kind: "output", Message: "write run output: " + err.Error()}
	}
	leaveIdle := func() error {
		err := emit(Event{Kind: "idle_finished"})
		idleStarted = time.Time{}
		return err
	}
	defer func() {
		if !idleStarted.IsZero() {
			if err := leaveIdle(); err != nil {
				result.Failure = outputFailure(err)
			}
		}
		result.Elapsed = time.Since(started)
		if result.Failure != nil {
			result.Reason = result.Failure.Kind
		}
		if err := emit(Event{Kind: "finished", Result: &result}); err != nil {
			result.Failure, result.Reason = outputFailure(err), "output"
		}
	}()
	interrupted := func() bool {
		if ctx.Err() == nil {
			return false
		}
		result.Failure = &Failure{Kind: "interrupted", Message: "run interrupted; inspect partial changes before retrying"}
		return true
	}
	for {
		previousParentTitle := ""
		if parent != nil {
			previousParentTitle = parent.Title
		}
		s := project.LoadContext(ctx, o.Root, project.ReadLimits{})
		if o.Query.Under != nil && s.ByID[*o.Query.Under] != nil {
			p := s.Summary(s.ByID[*o.Query.Under])
			parent = &p
		}
		if !announced {
			announced = true
			if err := emit(Event{Kind: "started"}); err != nil {
				result.Failure = outputFailure(err)
				return
			}
		}
		if interrupted() {
			return
		}
		if len(s.Diagnostics) > 0 {
			result.Failure = &Failure{Kind: "project", Message: "cannot select tickets from an invalid project", Diagnostics: s.Diagnostics}
			return
		}
		var matches []project.Summary
		if o.Query.Ticket != nil {
			t := s.ByID[*o.Query.Ticket]
			if t == nil {
				result.Failure = &Failure{Kind: "selection", Message: fmt.Sprintf("ticket %s not found in this project", *o.Query.Ticket)}
				return
			}
			matches = []project.Summary{s.Summary(t)}
		} else {
			if o.Query.Under != nil && (!ticket.IDPattern.MatchString(*o.Query.Under) || s.ByID[*o.Query.Under] == nil) {
				result.Failure = &Failure{Kind: "selection", Message: fmt.Sprintf("scope root %s not found in this project", *o.Query.Under)}
				return
			}
			matches = s.ScopedList(o.Query.All, o.Query.Ready, o.Query.Labels, o.Query.Under)
		}
		var next *project.Summary
		remaining := 0
		for _, match := range matches {
			if !processed[match.ID] {
				remaining++
				if next == nil {
					next = &match
				}
			}
		}
		previous := counts
		countsValid = true
		counts = SelectionCounts{Matches: len(matches), Processed: len(matches) - remaining, Unprocessed: remaining}
		if o.Query.Ticket != nil {
			counts.Scope = scopeCounts(matches)
		} else {
			counts.Scope = scopeCounts(s.ScopedList(true, false, o.Query.Labels, o.Query.Under))
		}
		if next == nil && o.Stream {
			kind := ""
			if idleStarted.IsZero() {
				idleStarted = time.Now()
				kind = "idle_started"
			} else if counts != previous || (parent != nil && parent.Title != previousParentTitle) {
				kind = "idle_changed"
			}
			if kind != "" {
				if err := emit(Event{Kind: kind}); err != nil {
					result.Failure = outputFailure(err)
					return
				}
			}
			if interrupted() {
				return
			}
			wait(ctx, o.PollInterval)
			continue
		}
		if !idleStarted.IsZero() {
			if err := leaveIdle(); err != nil {
				result.Failure = outputFailure(err)
				return
			}
		}
		if err := emit(Event{Kind: "selection"}); err != nil {
			result.Failure = outputFailure(err)
			return
		}
		if interrupted() {
			return
		}
		if next == nil {
			result.Reason = "drained"
			return
		}
		a := &Action{Sequence: result.Successful + 1, Started: time.Now(), RequiredStatus: o.ExpectStatus, CompletionCheck: "not_run", Ticket: *next, Command: append([]string{}, o.Command...)}
		// The executable is literal. Only argument values are ticket templates.
		for i := 1; i < len(a.Command); i++ {
			a.Command[i] = strings.ReplaceAll(a.Command[i], "{id}", next.ID)
		}
		result.LastAction = a
		if err := emit(Event{Kind: "action_started", Action: a}); err != nil {
			result.Failure = outputFailure(err)
			return
		}
		a.ExitCode, a.Signal, a.Failure = execute(ctx, o.Root, a.Command, o.Stdout, o.Stderr, o.HeartbeatInterval, func(activity Activity) error {
			a.Output, a.Elapsed = activity, time.Since(a.Started)
			return emit(Event{Kind: "heartbeat", Action: a})
		}, &a.Output)
		// Always reload after an exit-0 action, even without an expected status.
		// Deleted/invalid tickets cannot become successful processed IDs.
		if ctx.Err() == nil {
			current := project.LoadContext(ctx, o.Root, project.ReadLimits{})
			if len(current.Diagnostics) == 0 && current.ByID[next.ID] != nil {
				a.ObservedStatus = current.ByID[next.ID].Status
			}
			if a.Failure != nil {
				// Preserve the process failure; status is useful for explicit retry.
			} else if len(current.Diagnostics) > 0 {
				a.Failure = &Failure{Kind: "completion", Message: "project is invalid after the action", Diagnostics: current.Diagnostics}
			} else if t := current.ByID[next.ID]; t == nil {
				a.Failure = &Failure{Kind: "completion", Message: "selected ticket no longer exists in this project"}
			} else {
				a.ObservedStatus = t.Status
				if o.ExpectStatus != "" && t.Status != o.ExpectStatus {
					a.Failure = &Failure{Kind: "completion", Message: fmt.Sprintf("expected status %s; observed %s", o.ExpectStatus, t.Status)}
				}
			}
		}
		if interrupted() {
			a.Failure = result.Failure
		}
		a.Elapsed = time.Since(a.Started)
		ended := time.Now()
		a.Ended = &ended
		if a.Failure == nil {
			a.CompletionCheck = "passed"
		} else if a.Failure.Kind == "completion" {
			a.CompletionCheck = "failed"
		}
		if a.Failure == nil {
			processed[next.ID] = true
			result.Successful++
		}
		result.Failure = a.Failure
		if err := emit(Event{Kind: "action_finished", Action: a}); err != nil {
			result.Failure = outputFailure(err)
		}
		if result.Failure != nil || interrupted() {
			return
		}
		if o.Query.Ticket != nil {
			result.Reason = "ticket"
			return
		}
		if o.MaxTickets > 0 && result.Successful >= o.MaxTickets {
			result.Reason = "limit"
			return
		}
	}
}

func waitIdle(ctx context.Context, interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func scopeCounts(matches []project.Summary) (counts ScopeCounts) {
	for _, match := range matches {
		switch match.Status {
		case "todo":
			if len(match.Blockers) == 0 {
				counts.Ready++
			} else {
				counts.Waiting++
			}
		case "in-progress":
			counts.InProgress++
		case "blocked":
			counts.Blocked++
		case "done":
			counts.Done++
		case "canceled":
			counts.Canceled++
		}
	}
	return
}
