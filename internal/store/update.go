package store

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
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
	Updates                     []UpdateEntry
}

// UpdateEntry records the publication state of each recursive target.
type UpdateEntry struct {
	Ticket      *ticket.Ticket
	Publication string
}

type UpdateOptions struct {
	ticket.Changes
	Recursive bool
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
	return updateWithOptions(root, id, UpdateOptions{Changes: ticket.Changes{Title: title, Status: status}}, h)
}

func UpdateWithOptions(root, id string, opts UpdateOptions) Mutation {
	return updateWithOptions(root, id, opts, nil)
}

func updateWithOptions(root, id string, opts UpdateOptions, h *hooks) Mutation {
	if err := opts.Changes.Validate(); err != nil {
		return Mutation{Diagnostics: []diagnostic.Diagnostic{diagnostic.New("USAGE", err.Error(), "")}}
	}
	if opts.Recursive && (opts.Title != nil || opts.Status != nil || opts.Priority != nil) {
		return Mutation{Diagnostics: []diagnostic.Diagnostic{diagnostic.New("USAGE", "--recursive permits only label changes", "")}}
	}
	return withLock(root, func(s *project.Snapshot) (result Mutation) {
		result = Mutation{Snapshot: s, Diagnostics: []diagnostic.Diagnostic{}}
		t := s.ByID[id]
		if !ticket.IDPattern.MatchString(id) || t == nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic.New("NOT_FOUND", "ticket not found in this project", ""))
			return
		}
		result.Ticket = t
		ids := []string{id}
		if opts.Recursive {
			for child := range s.Descendants(id) {
				ids = append(ids, child)
			}
			sort.Strings(ids)
			result.Updates = make([]UpdateEntry, len(ids))
		}
		type prepared struct {
			ticket  *ticket.Ticket
			data    []byte
			staged  string
			info    os.FileInfo
			changed bool
		}
		batch := make([]prepared, len(ids))
		// Cleanup runs before withLock releases the project lock. Published files
		// are never rolled back; publish owns cleanup of its own staging name.
		defer func() {
			for _, item := range batch {
				if item.staged == "" {
					continue
				}
				err := h.hit("cleanup")
				if err == nil {
					err = os.Remove(item.staged)
				}
				if err != nil {
					result.Diagnostics = append(result.Diagnostics, diagnostic.New("CLEANUP_FAILED", "cleanup staging "+filepath.Base(item.staged)+": "+err.Error(), item.ticket.Path))
				}
			}
		}()
		// Validate every candidate before staging or publishing any file.
		for i, targetID := range ids {
			current := s.ByID[targetID]
			if opts.Recursive {
				result.Updates[i] = UpdateEntry{Ticket: current, Publication: "pending"}
			}
			data, changed, err := ticket.PatchChanges(current, opts.Changes)
			if err != nil {
				result.Diagnostics = append(result.Diagnostics, operationError(err, current.Path, false))
				return
			}
			batch[i] = prepared{ticket: current, data: data, changed: changed}
			if !changed {
				if opts.Recursive {
					result.Updates[i].Publication = "unchanged"
				}
				continue
			}
			candidate, ds := s.Candidate(current.Path, data)
			if len(ds) > 0 {
				result.Diagnostics = ds
				return
			}
			batch[i].ticket = candidate
		}
		// Stage the entire batch first: write/sync/close failures commit nothing.
		for i := range batch {
			item := &batch[i]
			if !item.changed {
				continue
			}
			mode := s.Files[item.ticket.Path].Info.Mode()
			var err error
			item.staged, err = stage(filepath.Join(root, ".wrk"), item.data, &mode, h)
			if err == nil {
				item.info, err = os.Lstat(item.staged)
			}
			if err != nil {
				result.Diagnostics = append(result.Diagnostics, operationError(err, item.ticket.Path, false))
				return
			}
		}
		for i := range batch {
			item := &batch[i]
			if !item.changed {
				continue
			}
			name := item.staged
			item.staged = "" // publish now owns this stage, including cleanup.
			publication := publish(name, filepath.Join(root, filepath.FromSlash(item.ticket.Path)), true, s.Compare, h)
			if publication.Committed {
				result.Committed, result.Changed = true, true
				if item.ticket.ID == id {
					result.Ticket = item.ticket
				}
				if opts.Recursive {
					result.Updates[i] = UpdateEntry{Ticket: item.ticket, Publication: "committed"}
				}
				// Advance only our expected file identity, never reload unrelated
				// inputs (which would accept an external edit between renames).
				s.Files[item.ticket.Path] = project.File{Data: item.data, Info: item.info}
				s.ByID[item.ticket.ID] = item.ticket
				for j, old := range s.Tickets {
					if old.ID == item.ticket.ID {
						s.Tickets[j] = item.ticket
						break
					}
				}
			}
			if publication.Err != nil {
				result.Diagnostics = append(result.Diagnostics, operationError(publication.Err, item.ticket.Path, publication.Committed))
				return
			}
		}
		return
	})
}
