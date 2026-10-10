package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	"wrk/internal/diagnostic"
	"wrk/internal/runner"
)

// Every lifecycle record uses the same versioned envelope on disk and stdout.
// No event/child transcript is retained in memory.
type runRecord struct {
	runner.Event
	InvocationCWD string `json:"invocation_cwd"`
	RunDirectory  string `json:"run_directory"`
	RunLog        string `json:"run_log"`
	Selection     string `json:"selection"`
	Ordering      string `json:"ordering"`
	ShowCommand   string `json:"show_command,omitempty"`
	RetryCommand  string `json:"retry_command,omitempty"`
}

type runOutput struct {
	mu                      sync.Mutex // serialize live stdout/stderr bytes with human events
	r                       Request
	root, cwd, dir, logPath string
	log, stdout, stderr     *os.File
	out, errout             io.Writer
	terminal, runningLine   bool
}

func newRunOutput(r Request, root, cwd string, out, errout io.Writer) (*runOutput, error) {
	base := r.LogDir
	if base == "" {
		base = filepath.Join(root, ".wrk-runs")
	} else if !filepath.IsAbs(base) {
		base = filepath.Join(cwd, base)
	}
	base, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		return nil, fmt.Errorf("create run logs: %w", err)
	}
	dir, err := os.MkdirTemp(base, time.Now().UTC().Format("20060102T150405.000000000Z")+"-")
	if err != nil {
		return nil, fmt.Errorf("create run directory: %w", err)
	}
	w := &runOutput{r: r, root: root, cwd: cwd, dir: dir, logPath: filepath.Join(dir, "run.jsonl"), out: out, errout: errout}
	w.log, err = os.OpenFile(w.logPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", w.logPath, err)
	}
	if f, ok := errout.(*os.File); ok && !r.JSON && !r.Verbose && os.Getenv("TERM") != "dumb" {
		_, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
		w.terminal = err == nil
	}
	return w, nil
}

type runChildWriter struct {
	recorder *runOutput
	stderr   bool
}

func (w runChildWriter) Write(p []byte) (int, error) {
	r := w.recorder
	r.mu.Lock()
	defer r.mu.Unlock()
	f := r.stdout
	if w.stderr {
		f = r.stderr
	}
	if f == nil {
		return 0, errors.New("child log is not open")
	}
	n, err := f.Write(p)
	if err != nil {
		return n, fmt.Errorf("write %s: %w", f.Name(), err)
	}
	if r.r.Verbose {
		if err := writeRun(r.errout, p); err != nil {
			return n, fmt.Errorf("live child output: %w", err)
		}
	}
	return n, nil
}

func writeRun(w io.Writer, p []byte) error {
	n, err := w.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return err
}

func closeRunFile(f **os.File) error {
	if *f == nil {
		return nil
	}
	file := *f
	*f = nil
	return errors.Join(file.Sync(), file.Close())
}

func (w *runOutput) closeChildren() error {
	return errors.Join(closeRunFile(&w.stdout), closeRunFile(&w.stderr))
}

func (w *runOutput) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return errors.Join(w.closeChildren(), closeRunFile(&w.log))
}

func (w *runOutput) event(e runner.Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if e.Kind == "action_started" {
		a := e.Action
		stem := fmt.Sprintf("%04d-%s", a.Sequence, a.Ticket.ID)
		a.StdoutLog, a.StderrLog = filepath.Join(w.dir, stem+".stdout.log"), filepath.Join(w.dir, stem+".stderr.log")
		var err error
		w.stdout, err = os.OpenFile(a.StdoutLog, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("create child stdout log: %w", err)
		}
		w.stderr, err = os.OpenFile(a.StderrLog, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("create child stderr log: %w", err)
		}
	}
	if e.Kind == "action_finished" || e.Kind == "finished" {
		if err := w.closeChildren(); err != nil {
			return fmt.Errorf("close child logs: %w", err)
		}
	}
	record := runRecord{Event: e, InvocationCWD: w.cwd, RunDirectory: w.dir, RunLog: w.logPath,
		Selection: describeSelection(e), Ordering: "ID ascending; first unprocessed match"}
	var failure *runner.Failure
	if e.Action != nil {
		failure = e.Action.Failure
	}
	if e.Result != nil {
		failure = e.Result.Failure
	}
	if failure != nil {
		a := e.Action
		if a == nil && e.Result != nil {
			a = e.Result.LastAction
		}
		// A later project/query failure is not a failed action to retry.
		if a != nil && (a.Failure != nil || a.Ended == nil) {
			record.ShowCommand = shellCommand([]string{"wrk", "--project", w.root, "show", a.Ticket.ID})
			retry := []string{"wrk", "--project", w.root, "run", "--ticket", a.Ticket.ID}
			if w.r.ExpectStatus != nil {
				retry = append(retry, "--expect-status", *w.r.ExpectStatus)
			}
			retry = append(retry, "--")
			// Re-run the original action template with this explicit ticket.
			retry = append(retry, w.r.ChildCommand...)
			record.RetryCommand = shellCommand(retry)
		}
	}
	errorsOut := []diagnostic.Diagnostic{}
	if failure != nil {
		errorsOut = append(errorsOut, diagnostic.New("RUN_"+strings.ToUpper(failure.Kind), failure.Message, ""))
		errorsOut = append(errorsOut, failure.Diagnostics...)
	}
	envelope := Envelope{SchemaVersion: 1, Command: "run", ProjectRoot: &w.root, OK: failure == nil, Result: record, Errors: errorsOut}
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := writeRun(w.log, data); err != nil {
		return fmt.Errorf("write %s: %w", w.logPath, err)
	}
	if e.Kind == "finished" {
		if err := closeRunFile(&w.log); err != nil {
			return fmt.Errorf("close run log: %w", err)
		}
	}
	if w.r.JSON {
		return writeRun(w.out, data)
	}
	return w.human(record, errorsOut)
}

func describeSelection(e runner.Event) string {
	q := e.Query
	if q.Ticket != nil {
		return "explicit ticket " + *q.Ticket + " (readiness not required)"
	}
	status := "active statuses: todo, in-progress, blocked"
	if q.All {
		status = "all statuses"
	}
	if q.Ready {
		status = "ready: todo with all dependencies done"
	}
	parts := []string{status}
	if len(q.Labels) > 0 {
		quoted := make([]string, len(q.Labels))
		for i, label := range q.Labels {
			quoted[i] = fmt.Sprintf("%q", label)
		}
		parts = append(parts, "all labels: "+strings.Join(quoted, ", "))
	}
	if q.Under != nil {
		parent := *q.Under
		if e.Parent != nil {
			parent = fmt.Sprintf("%q (%s)", e.Parent.Title, *q.Under)
		}
		parts = append(parts, "descendants of "+parent+" (parent excluded)")
	}
	return strings.Join(parts, " AND ")
}

func duration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

func emptyExplanation(e runner.Event) string {
	if e.Matches > 0 && e.Unprocessed == 0 {
		return "all current matches already processed; successful actions do not imply tickets are done"
	}
	return "no tickets currently match the execution filters"
}

func scopeExplanation(e runner.Event) string {
	c := e.Scope
	return fmt.Sprintf("Same label/descendant scope (status filters removed for diagnostics only): %d ready, %d todo waiting on dependencies, %d in-progress, %d manually blocked, %d done, %d canceled.", c.Ready, c.Waiting, c.InProgress, c.Blocked, c.Done, c.Canceled)
}

func activityText(e runner.Event) string {
	a := e.Action
	last := "no child output observed"
	if a.Output.LastOutput != nil {
		last = "last child output " + duration(e.Time.Sub(*a.Output.LastOutput)) + " ago"
	}
	return fmt.Sprintf("Running #%d %s; elapsed %s; %s", a.Sequence, a.Ticket.ID, duration(a.Elapsed), last)
}

func (w *runOutput) human(r runRecord, diagnostics []diagnostic.Diagnostic) error {
	e := r.Event
	var message string
	destination := w.errout
	switch e.Kind {
	case "started":
		mode := "ordinary; exit when no unprocessed matches remain"
		if e.Stream {
			mode = "stream; idle poll every " + e.PollInterval.String()
		}
		success := "exit 0 and valid existing ticket"
		if e.ExpectStatus != "" {
			success += "; persisted status " + e.ExpectStatus
		}
		limit := "unlimited"
		if e.MaxTickets > 0 {
			limit = fmt.Sprint(e.MaxTickets)
		}
		message = fmt.Sprintf("Project / child cwd: %s\nInvocation cwd: %s\nSelection: %s\nOrder: %s\nMode: %s; heartbeat every %s\nAction template: %s\nSuccess: %s; successful-action limit: %s\nRun log: %s", w.root, w.cwd, r.Selection, r.Ordering, mode, e.HeartbeatInterval, shellCommand(e.Command), success, limit, w.logPath)
	case "selection":
		message = fmt.Sprintf("Selection: %s; %d current matches, %d already processed, %d unprocessed candidates; %s.", r.Selection, e.Matches, e.Processed, e.Unprocessed, r.Ordering)
		if e.Unprocessed == 0 {
			message += "\n" + emptyExplanation(e) + ".\n" + scopeExplanation(e)
		}
	case "idle_started", "idle_changed":
		message = fmt.Sprintf("Waiting: no unprocessed matches (%d current matches, %d already processed); %s. Polling every %s; idle %s.\nSelection: %s\n%s", e.Matches, e.Processed, emptyExplanation(e), e.PollInterval, duration(e.Idle.Elapsed), r.Selection, scopeExplanation(e))
	case "idle_finished":
		message = fmt.Sprintf("Left idle after %s; %d unprocessed matches.", duration(e.Idle.Elapsed), e.Unprocessed)
	case "action_started":
		a := e.Action
		message = fmt.Sprintf("Starting %s: #%d %q (priority %s); %s.\nCommand: %s\nChild logs: %s ; %s", a.Ticket.ID, a.Sequence, a.Ticket.Title, a.Ticket.Priority, r.Ordering, shellCommand(a.Command), a.StdoutLog, a.StderrLog)
	case "heartbeat":
		message = activityText(e)
	case "action_finished":
		a := e.Action
		exit := "not started"
		if a.ExitCode != nil {
			exit = fmt.Sprintf("exit %d", *a.ExitCode)
		}
		outcome := "successful action"
		if a.Failure != nil {
			outcome = "process failure"
			if a.Failure.Kind == "completion" {
				outcome = "completion check failed"
			}
			if a.Failure.Kind == "interrupted" {
				outcome = "interrupted"
			}
			if a.Failure.Kind == "output" {
				outcome = "output failure"
			}
		}
		message = fmt.Sprintf("Finished %s: #%d %s; %s; elapsed %s", a.Ticket.ID, a.Sequence, outcome, exit, duration(a.Elapsed))
		if a.Signal != "" {
			message += "; signal " + a.Signal
		}
		if a.ObservedStatus != "" {
			message += "; observed status " + a.ObservedStatus
		}
		if a.RequiredStatus != "" {
			message += "; required status " + a.RequiredStatus + " (" + a.CompletionCheck + ")"
		}
	case "finished":
		destination = w.out
		result := e.Result
		reason := result.Reason
		switch reason {
		case "drained":
			reason = "no unprocessed matches remain in the current query"
		case "limit":
			reason = "successful-action limit reached"
		case "ticket":
			reason = "explicit ticket action succeeded"
		case "interrupted":
			reason = "interrupted by SIGINT/SIGTERM or cancellation"
		}
		if result.Failure != nil {
			reason += ": " + result.Failure.Message
		}
		message = fmt.Sprintf("Run %s: %d successful actions. Elapsed %s; stopped: %s.\nRun log: %s", result.Reason, result.Successful, duration(result.Elapsed), reason, w.logPath)
		if result.LastAction != nil && (result.LastAction.Failure != nil || result.LastAction.Ended == nil) {
			a := result.LastAction
			message += fmt.Sprintf("\nFailed/current ticket: %s; child logs: %s ; %s", a.Ticket.ID, a.StdoutLog, a.StderrLog)
		}
		if r.RetryCommand != "" {
			message += "\nShow ticket:\n" + r.ShowCommand + "\nInspect partial changes, then retry from the beginning:\n" + r.RetryCommand
		}
	}
	if w.runningLine {
		if err := writeRun(w.errout, []byte("\r\x1b[2K")); err != nil {
			return err
		}
		w.runningLine = false
	}
	if message != "" {
		text := Safe(message)
		if e.Kind == "heartbeat" && w.terminal {
			w.runningLine = true
			return writeRun(w.errout, []byte(text))
		}
		if err := writeRun(destination, []byte(text+"\n")); err != nil {
			return err
		}
	}
	// Keep errors as permanent milestones. The final event owns diagnostics so
	// an action failure isn't printed twice.
	if e.Kind == "finished" {
		for _, d := range diagnostics {
			if err := writeRun(w.errout, []byte(Safe(fmt.Sprintf("%s: %s %s\n", d.Code, d.Path, d.Message)))); err != nil {
				return err
			}
		}
	}
	return nil
}
