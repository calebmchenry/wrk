package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLimitsAndCancellation(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".wrk"), 0700); err != nil {
		t.Fatal(err)
	}
	source := "---\nid: wrk-00000001\ntitle: One\nstatus: todo\n---\n"
	for path, data := range map[string]string{ConfigPath: DefaultConfig, ".wrk/wrk-00000001.md": source} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, limits := range []ReadLimits{
		{FileBytes: 1}, {TotalBytes: int64(len(DefaultConfig) + len(source) - 1)}, {DirectoryEntries: 1},
	} {
		s := LoadContext(context.Background(), root, limits)
		if len(s.Diagnostics) != 1 || s.Diagnostics[0].Code != "RESOURCE_LIMIT" {
			t.Fatal(limits, s.Diagnostics)
		}
	}
	exact := ReadLimits{FileBytes: int64(max(len(DefaultConfig), len(source))), TotalBytes: int64(len(DefaultConfig) + len(source)), DirectoryEntries: 2}
	for _, limits := range []ReadLimits{{}, exact} {
		s := LoadContext(context.Background(), root, limits)
		if len(s.Diagnostics) > 0 || len(s.Tickets) != 1 {
			t.Fatal(s)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := LoadContext(ctx, root, ReadLimits{})
	if len(s.Diagnostics) != 1 || s.Diagnostics[0].Code != "CANCELED" {
		t.Fatal(s.Diagnostics)
	}
}
