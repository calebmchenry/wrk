//go:build darwin || linux

package store

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLockProcess(t *testing.T) {
	dir := os.Getenv("WRK_TEST_LOCK_DIR")
	if dir == "" {
		return
	}
	// Both children are launched before either is told to race first creation.
	fmt.Println("ready")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		os.Exit(10)
	}
	lock, err := Acquire(dir)
	if errors.Is(err, ErrBusy) {
		fmt.Println("busy")
		os.Exit(0)
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(11)
	}
	fmt.Println("held")
	scanner.Scan()
	lock.Close()
	os.Exit(0)
}
func TestTwoProcessesAndExitRelease(t *testing.T) {
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	type child struct {
		cmd  *exec.Cmd
		scan *bufio.Scanner
		in   *os.File
	}
	children := []child{}
	for i := 0; i < 2; i++ {
		cmd := exec.Command(exe, "-test.run=^TestLockProcess$")
		cmd.Env = append(os.Environ(), "WRK_TEST_LOCK_DIR="+dir)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stdin = read
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		read.Close()
		t.Cleanup(func() { cmd.Process.Kill(); write.Close() })
		scan := bufio.NewScanner(stdout)
		if !scan.Scan() || scan.Text() != "ready" {
			t.Fatal("child not ready")
		}
		children = append(children, child{cmd, scan, write})
	}
	for _, c := range children {
		fmt.Fprintln(c.in, "go")
	}
	held, busy := -1, 0
	for i, c := range children {
		if !c.scan.Scan() {
			t.Fatal("missing outcome")
		}
		switch c.scan.Text() {
		case "held":
			if held != -1 {
				t.Fatal("two simultaneous holders")
			}
			held = i
		case "busy":
			busy++
		default:
			t.Fatal(c.scan.Text())
		}
	}
	if held < 0 || busy != 1 {
		t.Fatal("expected one holder and one busy")
	}
	info, _ := os.Stat(filepath.Join(dir, ".lock"))
	// Replacing config never changes which inode owns the writer lock.
	os.WriteFile(filepath.Join(dir, "config-new"), []byte("new"), 0600)
	os.Rename(filepath.Join(dir, "config-new"), filepath.Join(dir, "config.yaml"))
	if lock, err := Acquire(dir); !errors.Is(err, ErrBusy) {
		if lock != nil {
			lock.Close()
		}
		t.Fatal("config replacement bypassed lock", err)
	}
	children[held].cmd.Process.Kill()
	children[held].cmd.Wait()
	for i, c := range children {
		if i != held {
			if err := c.cmd.Wait(); err != nil {
				t.Fatal(err)
			}
		}
	}
	lock, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	current, _ := os.Stat(filepath.Join(dir, ".lock"))
	if !os.SameFile(info, current) {
		t.Fatal("inode not persistent")
	}
}
func TestLockSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	os.WriteFile(target, []byte("unchanged"), 0600)
	os.Symlink(target, filepath.Join(dir, ".lock"))
	if lock, err := Acquire(dir); err == nil {
		lock.Close()
		t.Fatal("followed symlink")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "unchanged" {
		t.Fatal("truncated target")
	}
}
