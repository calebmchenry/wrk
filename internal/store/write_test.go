package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCompletePublicationAndFailures(t *testing.T) {
	for _, point := range []string{"", "write", "sync", "close", "link", "rename", "before_compare", "after_compare", "after_publish", "dir_sync", "cleanup"} {
		for _, replace := range []bool{false, true} {
			if point == "link" && replace || point == "rename" && !replace || point == "cleanup" && replace {
				continue
			}
			t.Run(point+map[bool]string{true: "-update", false: "-new"}[replace], func(t *testing.T) {
				dir := t.TempDir()
				dest := filepath.Join(dir, "wrk-12345678.md")
				if replace {
					if err := os.WriteFile(dest, []byte("before"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				h := &hooks{at: func(p string) error {
					if p == point {
						return errors.New("injected " + p)
					}
					return nil
				}}
				mode := os.FileMode(0600)
				s, err := stage(dir, []byte("complete contents"), &mode, h)
				result := Publication{Err: err}
				if err == nil {
					result = publish(s, dest, replace, func() error { return nil }, h)
				}
				wantCommit := point == "" || point == "after_publish" || point == "dir_sync" || point == "cleanup"
				if result.Committed != wantCommit || (result.Err != nil) != (point != "") {
					t.Fatalf("%+v", result)
				}
				got, readErr := os.ReadFile(dest)
				if wantCommit {
					if readErr != nil || string(got) != "complete contents" {
						t.Fatalf("%q %v", got, readErr)
					}
				} else if replace {
					if string(got) != "before" {
						t.Fatal("changed rejected update")
					}
				} else if !os.IsNotExist(readErr) {
					t.Fatal("partial creation")
				}
			})
		}
	}
}
func TestNoReplace(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "ticket")
	os.WriteFile(dest, []byte("existing"), 0600)
	s, err := stage(dir, []byte("candidate"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := publish(s, dest, false, nil, nil)
	if p.Committed || !errors.Is(p.Err, os.ErrExist) {
		t.Fatal(p)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "existing" {
		t.Fatal("overwritten")
	}
}
func TestPersistentLock(t *testing.T) {
	dir := t.TempDir()
	a, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(dir, ".lock"))
	if b, err := Acquire(dir); !errors.Is(err, ErrBusy) {
		if b != nil {
			b.Close()
		}
		t.Fatalf("wanted busy: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	after, _ := os.Stat(filepath.Join(dir, ".lock"))
	if !os.SameFile(info, after) {
		t.Fatal("lock inode changed")
	}
}
