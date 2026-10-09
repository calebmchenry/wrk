package project

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func selections(root string) []Selection {
	config := filepath.Join(root, ConfigPath)
	return []Selection{{Project: &root}, {Config: &config}}
}

func TestResolveExplicitBoundaries(t *testing.T) {
	for _, kind := range []string{"missing-root", "file-root", "missing-boundary", "file-boundary", "symlink-boundary", "valid"} {
		t.Run(kind, func(t *testing.T) {
			outer := seed(t)
			selected := filepath.Join(outer, "selected project")
			if kind != "missing-root" && kind != "file-root" {
				if err := os.Mkdir(selected, 0755); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "file-root":
				write(t, outer, "selected project", "file")
			case "file-boundary":
				write(t, selected, ".wrk", "file")
			case "symlink-boundary":
				if err := os.Symlink(filepath.Join(outer, ".wrk"), filepath.Join(selected, ".wrk")); err != nil {
					t.Fatal(err)
				}
			case "valid":
				write(t, selected, ConfigPath, DefaultConfig)
			}
			for _, selection := range selections(selected) {
				got, ds := Resolve(outer, selection)
				if got != selected || (len(ds) == 0) != (kind == "valid") {
					t.Fatalf("fell back or accepted invalid target: %s %+v", got, ds)
				}
			}
		})
	}
}

func TestResolvePathsAndDiscovery(t *testing.T) {
	outer := seed(t)
	selected := filepath.Join(outer, "selected project")
	write(t, selected, ConfigPath, DefaultConfig)
	child := filepath.Join(selected, "child")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		cwd, target, want string
	}{
		{child, "../..", outer},
		{outer, "selected project/child/..", selected},
		{t.TempDir(), selected, selected},
	} {
		for _, selection := range selections(tc.target) {
			root, ds := Resolve(tc.cwd, selection)
			if root != tc.want || len(ds) != 0 {
				t.Fatal(root, ds)
			}
		}
	}
	if root, ds := Resolve(child, Selection{}); root != selected || len(ds) != 0 {
		t.Fatal(root, ds)
	}
	// An invalid nearest boundary remains authoritative, while explicit selection
	// can deliberately select the outer project.
	write(t, child, ".wrk", "invalid")
	if root, ds := Resolve(child, Selection{}); root != child || !hasCode(ds, "INVALID_PROJECT") {
		t.Fatal(root, ds)
	}
	for _, selection := range selections(outer) {
		if root, ds := Resolve(child, selection); root != outer || len(ds) != 0 {
			t.Fatal(root, ds)
		}
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(selected, alias); err != nil {
		t.Fatal(err)
	}
	for _, selection := range selections(alias) {
		root, ds := Resolve(outer, selection)
		if root != alias || len(ds) != 0 || len(Load(root).Diagnostics) != 0 {
			t.Fatal("directory alias was expanded or rejected", root, ds)
		}
	}
}

func TestSelectedConfigAndTicketProtections(t *testing.T) {
	for _, kind := range []string{"missing", "invalid", "symlink", "directory", "fifo", "ticket-symlink", "ticket-fifo"} {
		t.Run(kind, func(t *testing.T) {
			root := seed(t)
			path := filepath.Join(root, ConfigPath)
			code := "INVALID_CONFIG"
			if kind == "ticket-symlink" || kind == "ticket-fifo" {
				path = filepath.Join(root, ".wrk/wrk-00000001.md")
				code = "INVALID_TICKET"
			} else if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "invalid":
				err = os.WriteFile(path, []byte("version: ["), 0600)
			case "symlink", "ticket-symlink":
				other := seed(t)
				err = os.Symlink(filepath.Join(other, ConfigPath), path)
			case "directory":
				err = os.Mkdir(path, 0755)
			case "fifo", "ticket-fifo":
				err = unix.Mkfifo(path, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, selection := range selections(root) {
				got, ds := Resolve(t.TempDir(), selection)
				if got != root || len(ds) != 0 || !hasCode(Load(got).Diagnostics, code) {
					t.Fatal(got, ds)
				}
			}
		})
	}
}

func TestResolveInvalidSelectors(t *testing.T) {
	root := seed(t)
	empty := ""
	for _, s := range []Selection{{Project: &root, Config: &root}, {Project: &empty}, {Config: &empty}} {
		if got, ds := Resolve(root, s); got != "" || !hasCode(ds, "USAGE") {
			t.Fatal(got, ds)
		}
	}
	for _, path := range []string{root, filepath.Join(root, "config.yaml"), filepath.Join(root, ".wrk/alternate.yaml")} {
		if got, ds := Resolve(root, Selection{Config: &path}); got != "" || !hasCode(ds, "INVALID_CONFIG") {
			t.Fatal(got, ds)
		}
	}
}
