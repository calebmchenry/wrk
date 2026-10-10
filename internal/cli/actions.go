package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"wrk/internal/diagnostic"
	"wrk/internal/runner"
)

func runActions(r Request, root, cwd string, out, errout io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Observe SIGPIPE so writes to a closed stdout/stderr report EPIPE instead
	// of terminating the runner before it can stop the child process group.
	pipeSignals := make(chan os.Signal, 1)
	signal.Notify(pipeSignals, syscall.SIGPIPE)
	defer signal.Stop(pipeSignals)
	recorder, err := newRunOutput(r, root, cwd, out, errout)
	if err != nil {
		e := Envelope{SchemaVersion: 1, Command: "run", ProjectRoot: &root, Errors: []diagnostic.Diagnostic{diagnostic.New("RUN_OUTPUT", err.Error(), "")}}
		return render(out, errout, r, e, "", 1)
	}
	o := runner.Options{
		Root: root, Command: r.ChildCommand, MaxTickets: r.MaxTickets,
		Stream: r.Stream, PollInterval: r.PollInterval, HeartbeatInterval: r.HeartbeatInterval,
		Query:  runner.Query{All: r.All, Ready: r.Ready, Labels: r.Labels, Under: r.Under, Ticket: r.RunTicket},
		Stdout: runChildWriter{recorder: recorder}, Stderr: runChildWriter{recorder: recorder, stderr: true},
		OnEvent: recorder.event,
	}
	if r.ExpectStatus != nil {
		o.ExpectStatus = *r.ExpectStatus
	}
	result := runner.Run(ctx, o)
	closeErr := recorder.close()
	if closeErr != nil || (result.Failure != nil && result.Failure.Kind == "output") {
		message := ""
		if result.Failure != nil {
			message = result.Failure.Message
		}
		if closeErr != nil {
			message += "; close logs: " + closeErr.Error()
		}
		// Best effort only: the event destination itself may have failed.
		fmt.Fprintln(errout, Safe("RUN_OUTPUT: "+message+"; logs: "+recorder.dir))
		return 1
	}
	return result.ExitCode()
}

// POSIX shell quoting is only for a copyable diagnostic, never execution.
func shellCommand(argv []string) string {
	parts := make([]string, len(argv))
	for i, arg := range argv {
		parts[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return strings.Join(parts, " ")
}
