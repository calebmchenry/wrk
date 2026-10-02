package project

import (
	"fmt"
	"os"
	"path/filepath"
	"wrk/internal/diagnostic"
)

func Discover(cwd string) (string, []diagnostic.Diagnostic) {
	root, err := filepath.Abs(cwd)
	if err != nil {
		return "", []diagnostic.Diagnostic{diagnostic.New("IO", err.Error(), "")}
	}
	for {
		entry := filepath.Join(root, ".wrk")
		info, err := os.Lstat(entry)
		if err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return root, []diagnostic.Diagnostic{diagnostic.New("INVALID_PROJECT", "nearest .wrk must be a real directory, not a file or symlink", ".wrk")}
			}
			return root, nil
		}
		if !os.IsNotExist(err) {
			return root, []diagnostic.Diagnostic{diagnostic.New("IO", fmt.Sprintf("inspect project boundary: %v", err), ".wrk")}
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}
	return "", []diagnostic.Diagnostic{diagnostic.New("NOT_FOUND", "no .wrk project found in this directory or its ancestors", "")}
}
