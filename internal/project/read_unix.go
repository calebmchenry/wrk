//go:build darwin || linux

package project

import (
	"context"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"os"
)

// readRegular rejects symlinks without following them and cannot block on a FIFO.
func readRegular(name string) ([]byte, os.FileInfo, error) {
	return readRegularContext(context.Background(), name, 0)
}

func readRegularContext(ctx context.Context, name string, maxBytes int64) ([]byte, os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
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
	if maxBytes > 0 && info.Size() > maxBytes {
		return nil, nil, errReadLimit
	}
	var reader io.Reader = contextReader{ctx, f}
	if maxBytes > 0 {
		reader = io.LimitReader(reader, maxBytes+1)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, nil, errReadLimit
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

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
