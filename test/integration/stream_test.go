//go:build darwin || linux

package integration

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Waiting text is a synchronization point: mutate only after the CLI has made
// its initial empty selection. Buffer access stays on the writer goroutine until
// Cmd.Wait has completed, and notification never blocks output delivery.
type idleOutput struct {
	buffer bytes.Buffer
	ready  chan struct{}
	once   sync.Once
}

func (w *idleOutput) Write(p []byte) (int, error) {
	n, err := w.buffer.Write(p)
	if strings.Contains(w.buffer.String(), "Waiting: no unprocessed matches") {
		w.once.Do(func() { close(w.ready) })
	}
	return n, err
}

func (w *idleOutput) String() string { return w.buffer.String() }

func TestRunStreamWhileIdle(t *testing.T) {
	for _, mode := range []string{"new work", "invalid project", "SIGINT", "SIGTERM"} {
		t.Run(mode, func(t *testing.T) {
			root, _ := runProject(t, 0)
			launch, _ := runProject(t, 1)
			interval := "10ms"
			if strings.HasPrefix(mode, "SIG") {
				interval = "1h" // Signals must interrupt, not await the next poll.
			}
			config, err := filepath.Rel(launch, filepath.Join(root, ".wrk/config.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			argv := append([]string{"--config", config, "run", "--stream", "--poll-interval=" + interval, "--ready", "--label=batch", "--expect-status=todo", "--max-tickets=2", "--"}, actionCommand(t, "noop")...)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, argv...)
			cmd.Dir = launch
			var stdout bytes.Buffer
			stderr := &idleOutput{ready: make(chan struct{})}
			cmd.Stdout, cmd.Stderr = &stdout, stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			finished := make(chan error, 1)
			go func() { finished <- cmd.Wait() }()
			select {
			case <-stderr.ready:
			case err := <-finished:
				t.Fatalf("stopped before idle: %v\n%s\n%s", err, stdout.String(), stderr.String())
			case <-ctx.Done():
				t.Fatal("did not enter idle")
			}
			wantCode, reason := 0, "limit"
			var ids []string
			switch mode {
			case "new work":
				for range 2 {
					ids = append(ids, newID(t, run(t, root, "", 0, "new", "Arrived while idle", "--label=batch", "--json")))
				}
			case "invalid project":
				put(t, root, ".wrk/config.yaml", "invalid: [")
				wantCode, reason = 1, "project"
			default:
				sig := syscall.SIGINT
				if mode == "SIGTERM" {
					sig = syscall.SIGTERM
				}
				if err := cmd.Process.Signal(sig); err != nil {
					t.Fatal(err)
				}
				wantCode, reason = 130, "interrupted"
			}
			select {
			case err := <-finished:
				code := 0
				if exit, ok := err.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else if err != nil {
					t.Fatal(err)
				}
				if code != wantCode {
					t.Fatalf("exit %d want %d: %s\n%s", code, wantCode, stdout.String(), stderr.String())
				}
			case <-time.After(5 * time.Second):
				t.Fatal("stream did not finish promptly")
			}
			if !strings.Contains(stdout.String(), fmt.Sprintf("Run %s: %d successful actions.", reason, len(ids))) || !strings.Contains(stderr.String(), "Left idle after") {
				t.Fatalf("missing lifecycle output: %s\n%s", stdout.String(), stderr.String())
			}
			got := actionIDs(t, root)
			slices.Sort(got)
			slices.Sort(ids)
			if !reflect.DeepEqual(got, ids) || len(actionIDs(t, launch)) != 0 {
				t.Fatalf("wrong project/tickets: got %v want %v", got, ids)
			}
		})
	}
}

func TestRunStreamJSONAndUsage(t *testing.T) {
	root, ids := runProject(t, 2)
	r, _, _ := runAction(t, root, 0, []string{"--stream", "--poll-interval=1h", "--max-tickets=2"}, actionCommand(t, "noop"))
	if r.Reason != "limit" || r.Successful != 2 || !reflect.DeepEqual(actionIDs(t, root), ids) {
		t.Fatalf("%+v", r)
	}
	for _, flags := range [][]string{
		{"--stream", "--poll-interval=0s"},
		{"--stream", "--poll-interval=-1s"},
		{"--stream", "--poll-interval=invalid"},
		{"--poll-interval=1s"},
		{"--stream", "--ticket=" + ids[0]},
	} {
		args := append([]string{"run", "--json"}, flags...)
		args = append(append(args, "--"), actionCommand(t, "noop")...)
		run(t, root, "", 2, args...)
	}
	if !reflect.DeepEqual(actionIDs(t, root), ids) {
		t.Fatal("invalid flags launched actions")
	}
}
