package store

import (
	"errors"
	"path/filepath"
	"strings"
	"wrk/internal/diagnostic"
	"wrk/internal/project"
	"wrk/internal/ticket"
)

type Mutation struct {
	Ticket                      *ticket.Ticket
	Snapshot                    *project.Snapshot
	Changed, Created, Committed bool
	Diagnostics                 []diagnostic.Diagnostic
}

func operationError(err error, path string, committed bool) diagnostic.Diagnostic {
	code := "IO"
	switch {
	case errors.Is(err, ErrBusy):
		code = "BUSY"
	case strings.Contains(err.Error(), "CONFLICT:"):
		code = "CONFLICT"
	case strings.Contains(err.Error(), "PRESERVATION_UNSUPPORTED:"):
		code = "PRESERVATION_UNSUPPORTED"
	case committed && strings.Contains(err.Error(), "durability uncertain:"):
		code = "DURABILITY_UNCERTAIN"
	case committed:
		code = "CLEANUP_FAILED"
	}
	return diagnostic.New(code, err.Error(), path)
}
func withLock(root string, fn func(*project.Snapshot) Mutation) (result Mutation) {
	result.Diagnostics = []diagnostic.Diagnostic{}
	lock, err := Acquire(filepath.Join(root, ".wrk"))
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, operationError(err, ".wrk/.lock", false))
		return
	}
	defer func() {
		if err := lock.Close(); err != nil {
			result.Diagnostics = append(result.Diagnostics, operationError(err, ".wrk/.lock", result.Committed))
		}
	}()
	s := project.Load(root)
	if len(s.Diagnostics) > 0 {
		result.Diagnostics = s.Diagnostics
		return
	}
	return fn(s)
}

func Update(root, id string, title, status *string) Mutation {
	return update(root, id, title, status, nil)
}
func update(root, id string, title, status *string, h *hooks) Mutation {
	return withLock(root, func(s *project.Snapshot) Mutation {
		result := Mutation{Snapshot: s, Diagnostics: []diagnostic.Diagnostic{}}
		t := s.ByID[id]
		if !ticket.IDPattern.MatchString(id) || t == nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic.New("NOT_FOUND", "ticket not found in this project", ""))
			return result
		}
		data, changed, err := ticket.Patch(t, title, status)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, operationError(err, t.Path, false))
			return result
		}
		result.Ticket = t
		if !changed {
			return result
		}
		candidate, ds := s.Candidate(t.Path, data)
		if len(ds) > 0 {
			result.Diagnostics = ds
			return result
		}
		mode := s.Files[t.Path].Info.Mode()
		staged, err := stage(filepath.Join(root, ".wrk"), data, &mode, h)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, operationError(err, t.Path, false))
			return result
		}
		publication := publish(staged, filepath.Join(root, filepath.FromSlash(t.Path)), true, s.Compare, h)
		result.Committed = publication.Committed
		result.Changed = publication.Committed
		if result.Committed {
			result.Ticket = candidate
		}
		if publication.Err != nil {
			result.Diagnostics = append(result.Diagnostics, operationError(publication.Err, t.Path, result.Committed))
		}
		return result
	})
}
