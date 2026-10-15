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
	RetryFile     string `json:"retry_file,omitempty"`
}

type runOutput struct {
	mu                      sync.Mutex // serialize live stdout/stderr bytes with human events
	r                       Request
	root, cwd, dir, logPath string
	log, stdout, stderr     *os.File
	out, errout             io.Writer
	livePartial             bool
	liveOutput              bool
	retryFile               string
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
		if len(p) > 0 {
			r.liveOutput = true
			r.livePartial = p[len(p)-1] != '\n'
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
			if w.retryFile == "" {
				if err := w.writeRetry(record.RetryCommand); err != nil {
					return err
				}
			}
			record.RetryFile = w.retryFile
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

// The retry is an inspectable artifact, never executed by the runner. Keep it
// separate from compact human output, including for multiline arguments.
func (w *runOutput) writeRetry(command string) error {
	path := filepath.Join(w.dir, "retry.sh")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create retry file: %w", err)
	}
	err = writeRun(f, []byte("#!/bin/sh\n# Inspect partial changes before retrying from the beginning.\n"+command+"\n"))
	err = errors.Join(err, closeRunFile(&f))
	if err != nil {
		return fmt.Errorf("write retry file: %w", err)
	}
	w.retryFile = path
	return nil
}
