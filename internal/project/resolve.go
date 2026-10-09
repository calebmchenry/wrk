package project

import (
	"fmt"
	"os"
	"path/filepath"
	"wrk/internal/diagnostic"
)

// Selection identifies an existing project. Nil selectors use cwd discovery.
type Selection struct {
	Project *string
	Config  *string
}

func (s Selection) Validate() error {
	if s.Project != nil && s.Config != nil {
		return fmt.Errorf("--project and --config conflict")
	}
	if s.Project != nil && *s.Project == "" {
		return fmt.Errorf("--project requires a nonempty path")
	}
	if s.Config != nil && *s.Config == "" {
		return fmt.Errorf("--config requires a nonempty path")
	}
	return nil
}

// Resolve selects a project boundary without changing cwd or creating files.
// Paths are cleaned and made absolute, but directory symlinks are not expanded.
// Callers must still load/validate the project (under the writer lock for writes).
// In particular, config and ticket file protections remain enforced by Load.
func Resolve(cwd string, selection Selection) (string, []diagnostic.Diagnostic) {
	if err := selection.Validate(); err != nil {
		return "", []diagnostic.Diagnostic{diagnostic.New("USAGE", err.Error(), "")}
	}
	if selection.Project == nil && selection.Config == nil {
		return Discover(cwd)
	}
	var path string
	if selection.Project != nil {
		path = *selection.Project
	} else {
		path = *selection.Config
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return "", []diagnostic.Diagnostic{diagnostic.New("IO", err.Error(), "")}
	}
	if selection.Config != nil {
		if filepath.Base(root) != "config.yaml" || filepath.Base(filepath.Dir(root)) != ".wrk" {
			return "", []diagnostic.Diagnostic{diagnostic.New("INVALID_CONFIG", "--config must name an existing project's .wrk/config.yaml", "")}
		}
		root = filepath.Dir(filepath.Dir(root))
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return root, []diagnostic.Diagnostic{diagnostic.New("INVALID_PROJECT", "selected project root must be an existing directory", "")}
	}
	info, err = os.Lstat(filepath.Join(root, ".wrk"))
	if err != nil {
		if os.IsNotExist(err) {
			return root, []diagnostic.Diagnostic{diagnostic.New("INVALID_PROJECT", "selected project must contain .wrk; explicit selection does not search ancestors or initialize a project", ".wrk")}
		}
		return root, []diagnostic.Diagnostic{diagnostic.New("IO", fmt.Sprintf("inspect selected project boundary: %v", err), ".wrk")}
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return root, []diagnostic.Diagnostic{diagnostic.New("INVALID_PROJECT", "selected .wrk must be a real directory, not a file or symlink", ".wrk")}
	}
	return root, nil
}
