//go:build darwin || linux

package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const shutdownGrace = 2 * time.Second

// Resolve relative executable paths and relative PATH entries from the child's
// project directory without changing this process's cwd or environment.
func executable(root, name string) (string, error) {
	if strings.ContainsRune(name, '/') {
		if !filepath.IsAbs(name) {
			name = filepath.Join(root, name)
		}
		return name, nil
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		if path, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
			return path, nil
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

// outputActivity is shared only by the two bounded os/exec copy goroutines.
// Notify immediately on a failed write: waiting for child exit could otherwise
// leave a silent child running indefinitely after its log became unwritable.
type outputActivity struct {
	mu       sync.Mutex
	activity Activity
	err      error
	failed   chan struct{}
}

type observedWriter struct {
	writer io.Writer
	state  *outputActivity
	stderr bool
}

func (w observedWriter) Write(p []byte) (int, error) {
	w.state.mu.Lock()
	now := time.Now()
	w.state.activity.LastOutput = &now
	if w.stderr {
		w.state.activity.StderrBytes += int64(len(p))
	} else {
		w.state.activity.StdoutBytes += int64(len(p))
	}
	w.state.mu.Unlock()
	n, err := w.writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.state.mu.Lock()
		if w.state.err == nil {
			w.state.err = err
			close(w.state.failed)
		}
		w.state.mu.Unlock()
	}
	return n, err
}

func (s *outputActivity) snapshot() (Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activity, s.err
}

func execute(ctx context.Context, root string, argv []string, stdout, stderr io.Writer, interval time.Duration, heartbeat func(Activity) error, activity *Activity) (*int, string, *Failure) {
	interrupted := func() *Failure {
		return &Failure{Kind: "interrupted", Message: "action interrupted; inspect partial changes before retrying"}
	}
	if ctx.Err() != nil {
		return nil, "", interrupted()
	}
	path, err := executable(root, argv[0])
	if err != nil {
		return nil, "", &Failure{Kind: "start", Message: err.Error()}
	}
	cmd := exec.Command(path, argv[1:]...)
	cmd.Args[0] = argv[0]
	cmd.Dir = root
	cmd.Env = os.Environ()
	state := &outputActivity{failed: make(chan struct{})}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	cmd.Stdout = observedWriter{writer: stdout, state: state}
	cmd.Stderr = observedWriter{writer: stderr, state: state, stderr: true}
	// Nil stdin is /dev/null, never the runner's interactive input.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// A descendant keeping copied output pipes open must not hang Wait forever.
	cmd.WaitDelay = shutdownGrace
	if err := cmd.Start(); err != nil {
		return nil, "", &Failure{Kind: "start", Message: err.Error()}
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var outputErr error
	stopChild := func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		timer := time.NewTimer(shutdownGrace)
		select {
		case err = <-waited:
		case <-timer.C:
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			err = <-waited
		}
		timer.Stop()
		// Even if the leader exited promptly, an ignoring descendant may remain.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
loop:
	for {
		select {
		case err = <-waited:
			break loop
		case <-ctx.Done():
			stopChild()
			break loop
		case <-state.failed:
			_, outputErr = state.snapshot()
			stopChild()
			break loop
		case <-ticker.C:
			current, _ := state.snapshot()
			if outputErr = heartbeat(current); outputErr != nil {
				stopChild()
				break loop
			}
		}
	}
	var writeErr error
	*activity, writeErr = state.snapshot()
	if outputErr == nil {
		outputErr = writeErr
	}
	code, signal := cmd.ProcessState.ExitCode(), ""
	if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		signal = status.Signal().String()
	}
	if outputErr != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return &code, signal, &Failure{Kind: "output", Message: "record child output: " + outputErr.Error()}
	}
	if ctx.Err() != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return &code, signal, interrupted()
	}
	if err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		kind := "wait"
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			kind = "exit"
		}
		return &code, signal, &Failure{Kind: kind, Message: fmt.Sprintf("child %s: %v", argv[0], err)}
	}
	return &code, signal, nil
}
