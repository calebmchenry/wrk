package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
	"wrk/internal/diagnostic"
	"wrk/internal/runner"
)

// Detect each destination independently. A redirected final summary must not
// inherit styling from terminal progress (or vice versa).
type runStyle struct {
	color    bool
	terminal bool
}

func styleForRun(destination io.Writer, json bool) runStyle {
	if json || os.Getenv("TERM") == "dumb" {
		return runStyle{}
	}
	f, ok := destination.(*os.File)
	if !ok {
		return runStyle{}
	}
	_, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	_, noColor := os.LookupEnv("NO_COLOR")
	return runStyle{terminal: err == nil, color: err == nil && !noColor}
}

// Escape data before adding our own SGR sequences. All variable values occupy
// one logical line, even when a title, path, argument, or diagnostic has newlines.
func runLine(s string) string {
	return strings.NewReplacer("\n", `\n`, "\t", `\t`).Replace(Safe(s))
}

func (s runStyle) emphasis(value string) string {
	value = runLine(value)
	if s.color {
		return "\x1b[1m" + value + "\x1b[0m"
	}
	return value
}

func (s runStyle) state(label string) string {
	code := "34" // running/start: blue
	switch label {
	case "WAITING", "RESUMED":
		code = "35" // magenta, not a silence warning
	case "OK", "DONE":
		code = "32"
	case "FAILED", "STOPPED":
		code = "31"
	}
	if s.color {
		return "\x1b[1;" + code + "m" + label + "\x1b[0m"
	}
	return label
}

func (s runStyle) newline() string {
	if s.terminal {
		// Explicit carriage return also works when another terminal user has
		// disabled ONLCR/OPOST. No cursor movement or row erasure is used.
		return "\r\n"
	}
	return "\n"
}

func commandDescription(command []string) string {
	if len(command) == 0 {
		return "(no command)"
	}
	name := []rune(command[0])
	if len(name) > 64 {
		name = append(name[:61], '.', '.', '.')
	}
	return fmt.Sprintf("%s (%d arguments; full argv in run log)", runLine(string(name)), len(command)-1)
}

func activityText(e runner.Event, s runStyle) string {
	a := e.Action
	last := s.emphasis("no output yet")
	if a.Output.LastOutput != nil {
		last = "last output " + s.emphasis(duration(e.Time.Sub(*a.Output.LastOutput))) + " ago"
	}
	return fmt.Sprintf("%s  %s | elapsed %s | %s", s.state("RUNNING"), runLine(a.Ticket.ID), s.emphasis(duration(a.Elapsed)), last)
}

func (w *runOutput) human(r runRecord, diagnostics []diagnostic.Diagnostic) error {
	e := r.Event
	destination := w.errout
	if e.Kind == "finished" {
		destination = w.out
	}
	s := styleForRun(destination, w.r.JSON)
	var message string
	switch e.Kind {
	case "started":
		mode := "ordinary; exit when drained"
		if e.Stream {
			mode = "stream; poll " + e.PollInterval.String()
		}
		success := "child exit 0 + valid existing ticket"
		if e.ExpectStatus != "" {
			success += "; status " + runLine(e.ExpectStatus)
		}
		limit := "unlimited"
		if e.MaxTickets > 0 {
			limit = fmt.Sprint(e.MaxTickets)
		}
		message = fmt.Sprintf("%s  %s\nProject: %s\nSelect: %s\nMode: %s | limit %s | heartbeat %s\nSuccess: %s\nRun log (commands and details):\n  %s",
			s.state("RUN"), commandDescription(e.Command), runLine(w.root), runLine(r.Selection), mode, limit, e.HeartbeatInterval, success, runLine(w.logPath))
	case "selection":
		// Idle records own empty stream selections; don't print them twice.
		if e.Stream && e.Unprocessed == 0 {
			break
		}
		message = fmt.Sprintf("QUEUE  %d matches | %d processed | %d candidates (ID order)", e.Matches, e.Processed, e.Unprocessed)
		if e.Unprocessed == 0 {
			message += "\n  " + emptyExplanation(e) + ".\n  " + scopeExplanation(e)
		}
	case "idle_started", "idle_changed":
		message = fmt.Sprintf("%s  %d matches | %d processed | idle %s | poll %s\n  %s.\n  %s",
			s.state("WAITING"), e.Matches, e.Processed, s.emphasis(duration(e.Idle.Elapsed)), e.PollInterval, emptyExplanation(e), scopeExplanation(e))
	case "idle_finished":
		message = fmt.Sprintf("%s  waited %s | %d candidates", s.state("RESUMED"), s.emphasis(duration(e.Idle.Elapsed)), e.Unprocessed)
		if e.Unprocessed == 0 {
			message = fmt.Sprintf("WAIT END  waited %s", s.emphasis(duration(e.Idle.Elapsed)))
		}
	case "action_started":
		a := e.Action
		message = fmt.Sprintf("%s  %s | #%d | %s", s.state("START"), runLine(a.Ticket.ID), a.Sequence, s.emphasis(a.Ticket.Title))
	case "heartbeat":
		message = activityText(e, s)
	case "action_finished":
		a := e.Action
		exit := "child not started"
		if a.ExitCode != nil {
			exit = fmt.Sprintf("child exit %d", *a.ExitCode)
		}
		label, outcome := "OK", "action succeeded"
		if a.Failure != nil {
			label, outcome = "FAILED", "process failure"
			switch a.Failure.Kind {
			case "completion":
				outcome = "completion check failed"
			case "interrupted":
				label, outcome = "STOPPED", "interrupted"
			case "output":
				outcome = "output failure"
			}
		}
		message = fmt.Sprintf("%s  %s | %s | elapsed %s | %s", s.state(label), runLine(a.Ticket.ID), exit, s.emphasis(duration(a.Elapsed)), outcome)
		if a.Signal != "" {
			message += " | signal " + runLine(a.Signal)
		}
		if a.ObservedStatus != "" {
			message += "\n  Status: " + runLine(a.ObservedStatus)
			if a.RequiredStatus != "" {
				message += " (expected " + runLine(a.RequiredStatus) + ")"
			}
		}
	case "finished":
		result := e.Result
		label, reason := "DONE", result.Reason
		switch reason {
		case "drained":
			reason = "no unprocessed matches remain"
		case "limit":
			reason = "successful-action limit reached"
		case "ticket":
			reason = "explicit ticket action succeeded"
		}
		if result.Failure != nil {
			label = "FAILED"
			if result.Failure.Kind == "interrupted" {
				label = "STOPPED"
			}
			reason = "RUN_" + strings.ToUpper(result.Failure.Kind) + ": " + result.Failure.Message
		}
		message = fmt.Sprintf("%s  %s successful actions | elapsed %s\n  %s", s.state(label), s.emphasis(fmt.Sprint(result.Successful)), s.emphasis(duration(result.Elapsed)), runLine(reason))
		if result.LastAction != nil && (result.LastAction.Failure != nil || result.LastAction.Ended == nil) {
			a := result.LastAction
			exit := "child not started"
			if a.ExitCode != nil {
				exit = fmt.Sprintf("child exit %d", *a.ExitCode)
			}
			message += fmt.Sprintf("\n  Ticket: %s | %s | elapsed %s", runLine(a.Ticket.ID), exit, s.emphasis(duration(a.Elapsed)))
			message += "\nLogs (files in this directory):\n  " + runLine(w.dir) + "\n  Run:    " + runLine(filepath.Base(w.logPath)) + "\n  Stdout: " + runLine(filepath.Base(a.StdoutLog)) + "\n  Stderr: " + runLine(filepath.Base(a.StderrLog))
		} else {
			message += "\nRun log:\n  " + runLine(w.logPath)
		}
		if r.RetryCommand != "" {
			message += "\nRecovery:\n  Show ticket: " + runLine(r.ShowCommand) + "\n  Inspect partial changes before retrying from the beginning.\n  Exact retry (view/copy): cat " + runLine(shellCommand([]string{r.RetryFile}))
		}
		// The headline owns the primary failure. Keep underlying validation
		// diagnostics, without repeating the same RUN_* message on stderr.
		for _, d := range diagnostics {
			if result.Failure != nil && d.Code == "RUN_"+strings.ToUpper(result.Failure.Kind) {
				continue
			}
			message += "\n  " + runLine(fmt.Sprintf("%s: %s %s", d.Code, d.Path, d.Message))
		}
	}
	if message == "" {
		return nil
	}
	// A child may finish a write without LF, then stay silent or exit. Separate
	// our next record without altering raw logs or JSON/verbose byte streams.
	if w.liveOutput {
		childStyle := styleForRun(w.errout, false)
		separator := ""
		if w.livePartial {
			separator = childStyle.newline()
		} else if childStyle.terminal {
			// Raw child LF may not have returned the cursor to column zero.
			separator = "\r"
		}
		if separator != "" {
			if err := writeRun(w.errout, []byte(separator)); err != nil {
				return err
			}
		}
		w.liveOutput, w.livePartial = false, false
	}
	return writeRun(destination, []byte(strings.ReplaceAll(message+"\n", "\n", s.newline())))
}
