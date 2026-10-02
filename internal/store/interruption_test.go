//go:build darwin || linux

package store

import (
	"bufio"
	"bytes"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"wrk/internal/project"
)

func TestInterruptedProcess(t *testing.T) {
	root := os.Getenv("WRK_TEST_INTERRUPT_ROOT")
	if root == "" {
		return
	}
	point := os.Getenv("WRK_TEST_INTERRUPT_POINT")
	status := "done"
	h := &hooks{at: func(p string) error {
		if p == point {
			fmt.Println("checkpoint")
			bufio.NewScanner(os.Stdin).Scan()
			os.Exit(12)
		}
		return nil
	}}
	if os.Getenv("WRK_TEST_INTERRUPT_NEW") == "true" {
		create(root, CreateOptions{Title: "Created"}, bytes.NewReader([]byte{0, 0, 0, 1}), h)
	} else {
		update(root, testID, nil, &status, h)
	}
	os.Exit(13)
}
func TestInterruptionBeforeAndAfterPublication(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range []string{"before_compare", "after_publish"} {
		for _, newTicket := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-new-%t", point, newTicket), func(t *testing.T) {
				root := testProject(t)
				cmd := exec.Command(exe, "-test.run=^TestInterruptedProcess$")
				cmd.Env = append(os.Environ(), "WRK_TEST_INTERRUPT_ROOT="+root, "WRK_TEST_INTERRUPT_POINT="+point, fmt.Sprintf("WRK_TEST_INTERRUPT_NEW=%t", newTicket))
				stdout, err := cmd.StdoutPipe()
				if err != nil {
					t.Fatal(err)
				}
				stdin, err := cmd.StdinPipe()
				if err != nil {
					t.Fatal(err)
				}
				defer stdin.Close()
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { cmd.Process.Kill() })
				scan := bufio.NewScanner(stdout)
				if !scan.Scan() || scan.Text() != "checkpoint" {
					t.Fatal("no checkpoint")
				}
				cmd.Process.Kill()
				cmd.Wait()
				s := project.Load(root)
				if len(s.Diagnostics) > 0 {
					t.Fatal(s.Diagnostics)
				}
				if newTicket {
					want := 1
					if point == "after_publish" {
						want = 2
					}
					if len(s.Tickets) != want {
						t.Fatal("partial or missing creation")
					}
				} else {
					want := "todo"
					if point == "after_publish" {
						want = "done"
					}
					if s.ByID[testID].Status != want {
						t.Fatal("publication point incorrect")
					}
				}
				lock, err := Acquire(filepath.Join(root, ".wrk"))
				if err != nil {
					t.Fatal("lock not released", err)
				}
				lock.Close()
				entries, _ := filepath.Glob(filepath.Join(root, ".wrk/.wrk-stage-*"))
				if (newTicket || point == "before_compare") && len(entries) == 0 {
					t.Fatal("expected ignored residual staging")
				}
			})
		}
	}
}
func TestUmaskAndSpecialModes(t *testing.T) {
	// This package never runs tests in parallel: umask is process-global.
	old := unix.Umask(0077)
	defer unix.Umask(old)
	root := t.TempDir()
	r := Init(root)
	if len(r.Diagnostics) > 0 {
		t.Fatal(r.Diagnostics)
	}
	created := Create(root, CreateOptions{Title: "Private"})
	if len(created.Diagnostics) > 0 {
		t.Fatal(created.Diagnostics)
	}
	for _, path := range []string{project.ConfigPath, created.Ticket.Path, ".wrk/.lock"} {
		info, err := os.Stat(filepath.Join(root, path))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("%s: %v %v", path, info, err)
		}
	}
	path := filepath.Join(root, created.Ticket.Path)
	os.Chmod(path, 0400)
	title := "Updated"
	updated := Update(root, created.Ticket.ID, &title, nil)
	if len(updated.Diagnostics) > 0 {
		t.Fatal(updated.Diagnostics)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0400 {
		t.Fatal("widened read-only mode")
	}
}
