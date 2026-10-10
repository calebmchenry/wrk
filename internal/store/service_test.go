package store

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wrk/internal/project"
	"wrk/internal/ticket"
)

func TestServiceMutationLimitsAndCancellation(t *testing.T) {
	for _, kind := range []string{"load", "candidate file", "candidate total", "cancel load", "cancel publication", "growth before publication"} {
		t.Run(kind, func(t *testing.T) {
			root := testProject(t)
			before := project.Load(root)
			var total int64
			for _, file := range before.Files {
				total += int64(len(file.Data))
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			limits := project.ReadLimits{FileBytes: 1 << 20, TotalBytes: 2 << 20, DirectoryEntries: 100}
			body := strings.Repeat("x", 4096)
			var h *hooks
			want := "RESOURCE_LIMIT"
			switch kind {
			case "load":
				limits.FileBytes = 1
			case "candidate file":
				limits.FileBytes = 2048
			case "candidate total":
				limits.TotalBytes = total + 100
			case "cancel load":
				cancel()
				want = "CANCELED"
			case "cancel publication":
				want = "CONFLICT"
				h = &hooks{at: func(step string) error {
					if step == "before_compare" {
						cancel()
					}
					return nil
				}}
			case "growth before publication":
				want = "CONFLICT"
				h = &hooks{at: func(step string) error {
					if step == "before_compare" {
						return os.WriteFile(filepath.Join(root, project.ConfigPath), []byte(strings.Repeat("x", 2<<20)), 0600)
					}
					return nil
				}}
			}
			revision := ticket.Revision(before.ByID[testID].Source)
			m := updateContext(ctx, root, testID, UpdateOptions{Changes: ticket.Changes{Body: &body}, ExpectedRevision: &revision}, limits, h)
			requireMutationCode(t, m, want)
			data, err := os.ReadFile(filepath.Join(root, before.ByID[testID].Path))
			if err != nil || string(data) != string(before.ByID[testID].Source) {
				t.Fatal("rejected service update changed target", err)
			}
			if kind != "growth before publication" {
				if err := before.Compare(); err != nil {
					t.Fatal(err)
				}
			}
			// Creation follows the same bounded read/candidate/publication path.
			if kind != "growth before publication" && kind != "cancel publication" {
				m = createContext(ctx, root, CreateOptions{Title: "Bounded", Body: []byte(body)}, limits, rand.Reader, nil)
				requireMutationCode(t, m, want)
				if err := before.Compare(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
