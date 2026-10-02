package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Hooks are private, per-operation deterministic fault seams used by store tests.
// Production never reads fault settings from arguments or environment variables.
type hooks struct{ at func(string) error }

func (h *hooks) hit(point string) error {
	if h != nil && h.at != nil {
		return h.at(point)
	}
	return nil
}

type Publication struct {
	Committed bool
	collision bool
	Err       error
}

func stage(dir string, data []byte, mode *os.FileMode, h *hooks) (name string, err error) {
	var f *os.File
	for attempt := 0; attempt < 128; attempt++ {
		token := make([]byte, 12)
		if _, err = rand.Read(token); err != nil {
			return "", err
		}
		name = filepath.Join(dir, ".wrk-stage-"+hex.EncodeToString(token))
		f, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		break
	}
	if err != nil {
		return "", err
	}
	defer func() {
		if f != nil {
			err = errors.Join(err, f.Close())
		}
		if err != nil {
			err = errors.Join(err, os.Remove(name))
			name = ""
		}
	}()
	if mode != nil {
		if err = f.Chmod(mode.Perm()); err != nil {
			return name, err
		}
	}
	if err = h.hit("write"); err != nil {
		return name, err
	}
	n, e := f.Write(data)
	if e != nil {
		return name, e
	}
	if n != len(data) {
		return name, io.ErrShortWrite
	}
	if err = h.hit("sync"); err != nil {
		return name, err
	}
	if err = f.Sync(); err != nil {
		return name, err
	}
	if err = h.hit("close"); err != nil {
		return name, err
	}
	err = f.Close()
	f = nil
	return name, err
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}

func publish(staged, target string, replace bool, compare func() error, h *hooks) (p Publication) {
	defer func() {
		if staged != "" {
			err := h.hit("cleanup")
			if err == nil {
				err = os.Remove(staged)
			}
			if err != nil {
				p.Err = errors.Join(p.Err, fmt.Errorf("cleanup staging %s: %w", filepath.Base(staged), err))
				p.collision = false
			}
		}
	}()
	if err := h.hit("before_compare"); err != nil {
		return Publication{Err: err}
	}
	if compare != nil {
		if err := compare(); err != nil {
			return Publication{Err: err}
		}
	}
	if err := h.hit("after_compare"); err != nil {
		return Publication{Err: err}
	}
	if replace {
		if err := h.hit("rename"); err != nil {
			return Publication{Err: err}
		}
		if err := os.Rename(staged, target); err != nil {
			return Publication{Err: err}
		}
		staged = ""
	} else {
		if err := h.hit("link"); err != nil {
			return Publication{Err: err}
		}
		if err := os.Link(staged, target); err != nil {
			return Publication{Err: err, collision: errors.Is(err, os.ErrExist)}
		}
	}
	p.Committed = true
	if err := h.hit("after_publish"); err != nil {
		p.Err = err
		return
	}
	if err := h.hit("dir_sync"); err != nil {
		p.Err = fmt.Errorf("durability uncertain: %w", err)
		return
	}
	if err := syncDir(filepath.Dir(target)); err != nil {
		p.Err = fmt.Errorf("durability uncertain: %w", err)
	}
	return
}
