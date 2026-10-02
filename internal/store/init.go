package store

import (
	"fmt"
	"os"
	"path/filepath"
	"wrk/internal/diagnostic"
	"wrk/internal/project"
)

type Initialization struct {
	Root               string
	Created, Committed bool
	Diagnostics        []diagnostic.Diagnostic
}

func Init(target string) Initialization { return initialize(target, nil) }
func initialize(target string, h *hooks) (result Initialization) {
	result.Diagnostics = []diagnostic.Diagnostic{}
	root, err := filepath.Abs(target)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, operationError(err, "", false))
		return
	}
	result.Root = root
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		result.Diagnostics = append(result.Diagnostics, diagnostic.New("INVALID_TARGET", "init requires an existing target directory", ""))
		return
	}
	dir := filepath.Join(root, ".wrk")
	if err = os.Mkdir(dir, 0777); err != nil {
		code := "IO"
		if os.IsExist(err) {
			code = "ALREADY_EXISTS"
		}
		result.Diagnostics = append(result.Diagnostics, diagnostic.New(code, "cannot initialize .wrk: "+err.Error(), ".wrk"))
		return
	}
	ownedDir, err := os.Lstat(dir)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, operationError(err, ".wrk", false))
		return
	}
	defer func() {
		if result.Committed {
			return
		}
		// Never recursively delete: another process may have populated our directory.
		current, err := os.Lstat(dir)
		if err == nil && os.SameFile(ownedDir, current) {
			if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
				if err := os.Remove(dir); err != nil {
					result.Diagnostics = append(result.Diagnostics, operationError(err, ".wrk", false))
				}
			}
		}
	}()
	staged, err := stage(dir, []byte(project.DefaultConfig), nil, h)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, operationError(err, project.ConfigPath, false))
		return
	}
	p := publish(staged, filepath.Join(root, project.ConfigPath), false, func() error {
		current, err := os.Lstat(dir)
		if err != nil || !os.SameFile(ownedDir, current) {
			return fmt.Errorf("CONFLICT: initialization directory changed")
		}
		return nil
	}, h)
	result.Committed = p.Committed
	result.Created = p.Committed
	if p.Err != nil {
		result.Diagnostics = append(result.Diagnostics, operationError(p.Err, project.ConfigPath, p.Committed))
	}
	if p.Committed {
		if err := syncDir(root); err != nil {
			result.Diagnostics = append(result.Diagnostics, operationError(fmt.Errorf("durability uncertain: %w", err), ".wrk", true))
		}
	}
	return
}
