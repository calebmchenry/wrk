//go:build darwin || linux

package store

import (
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

var ErrBusy = errors.New("BUSY: another wrk writer holds the project lock")

type Lock struct{ file *os.File }

func Acquire(dir string) (*Lock, error) {
	name := filepath.Join(dir, ".lock")
	fd, err := unix.Open(name, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0666)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	f := os.NewFile(uintptr(fd), name)
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("lock must be a regular file")
	}
	if err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrBusy
		}
		return nil, err
	}
	current, err := os.Lstat(name)
	if err != nil || !os.SameFile(info, current) {
		f.Close()
		return nil, fmt.Errorf("lock identity changed")
	}
	return &Lock{f}, nil
}
func (l *Lock) Close() error { return l.file.Close() }
