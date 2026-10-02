//go:build darwin || linux

package project

import (
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"os"
)

// readRegular rejects symlinks without following them and cannot block on a FIFO.
func readRegular(name string) ([]byte, os.FileInfo, error) {
	fd, err := unix.Open(name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("expected a regular file, not a symlink or special file")
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}
	current, err := os.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !os.SameFile(info, current) {
		return nil, nil, fmt.Errorf("file identity changed while reading")
	}
	return data, info, nil
}
