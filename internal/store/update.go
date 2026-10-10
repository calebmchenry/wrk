package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
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

// UpdateEntry records the publication state of each batch target.
type UpdateEntry struct {
	Ticket      *ticket.Ticket
	Publication string
}

type UpdateOptions struct {
	ticket.Changes
	Recursive bool
	// ExpectedRevision requires a current single-item source revision, even for no-ops.
	// Nil opts out; a supplied empty revision never matches a ticket.
	ExpectedRevision *string
	// ExpectedRelatedRevision checks the symmetric link set, including incoming edges.
	ExpectedRelatedRevision *string
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
func withLock(ctx context.Context, limits project.ReadLimits, root string, precondition func(*project.Snapshot) []diagnostic.Diagnostic, fn func(*project.Snapshot) Mutation) (result Mutation) {
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
	s := project.LoadContext(ctx, root, limits)
	// Resource/cancellation failures must not be misreported as a missing draft.
	for _, d := range s.Diagnostics {
		if d.Code == "RESOURCE_LIMIT" || d.Code == "CANCELED" {
			result.Diagnostics = s.Diagnostics
			return
		}
	}
	// Check against the locked read before general diagnostics so deletion or an
	// invalid external rewrite still reports a recognizable stale-edit failure.
	if precondition != nil {
		if ds := precondition(s); len(ds) > 0 {
			result.Diagnostics = ds
			return
		}
	}
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

// UpdateContext retains the CLI mutation protocol with bounded service reads.
func UpdateContext(ctx context.Context, root, id string, opts UpdateOptions, limits project.ReadLimits) Mutation {
	return updateContext(ctx, root, id, opts, limits, nil)
}
func updateWithOptions(root, id string, opts UpdateOptions, h *hooks) Mutation {
	return updateContext(context.Background(), root, id, opts, project.ReadLimits{}, h)
}
func updateContext(ctx context.Context, root, id string, opts UpdateOptions, limits project.ReadLimits, h *hooks) Mutation {
	if err := opts.Changes.Validate(); err != nil {
		code := "USAGE"
		if errors.Is(err, ticket.ErrInvalidBody) {
			code = "INVALID_BODY"
		}
		return Mutation{Diagnostics: []diagnostic.Diagnostic{diagnostic.New(code, err.Error(), "")}}
	}
	if opts.Recursive && opts.Changes.HasNonLabelChanges() {
		return Mutation{Diagnostics: []diagnostic.Diagnostic{diagnostic.New("USAGE", "--recursive permits only label changes", "")}}
	}
	if opts.Recursive && (opts.ExpectedRevision != nil || opts.ExpectedRelatedRevision != nil) {
		return Mutation{Diagnostics: []diagnostic.Diagnostic{diagnostic.New("USAGE", "expected revision is only supported for single-ticket updates", "")}}
	}
	return withLock(ctx, limits, root, func(s *project.Snapshot) []diagnostic.Diagnostic {
		if ds := checkRevision(s, id, opts.ExpectedRevision); len(ds) > 0 {
			return ds
		}
		if opts.ExpectedRelatedRevision != nil && len(s.Diagnostics) == 0 {
			if s.ByID[id] == nil {
				return []diagnostic.Diagnostic{diagnostic.New("NOT_FOUND", "ticket not found in this project", "")}
			}
			if *opts.ExpectedRelatedRevision != s.RelatedRevision(id) {
				return []diagnostic.Diagnostic{diagnostic.New("CONFLICT", "related items changed since they were read; reload and review before retrying", s.ByID[id].Path)}
			}
		}
		return nil
	}, func(s *project.Snapshot) (result Mutation) {
		result = Mutation{Snapshot: s, Diagnostics: []diagnostic.Diagnostic{}}
		t := s.ByID[id]
		if !ticket.IDPattern.MatchString(id) || t == nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic.New("NOT_FOUND", "ticket not found in this project", ""))
			return
		}
		result.Ticket = t
		changes := map[string]ticket.Changes{id: opts.Changes}
		if opts.Recursive {
			for child := range s.Descendants(id) {
				changes[child] = opts.Changes
			}
		} else if opts.HasRelatedChanges() {
			var ds []diagnostic.Diagnostic
			changes, ds = relatedChanges(s, id, opts.Changes)
			if len(ds) > 0 {
				result.Diagnostics = ds
				return
			}
		}
		ids := make([]string, 0, len(changes))
		for target := range changes {
			ids = append(ids, target)
		}
		sort.Strings(ids)
		tracked := opts.Recursive || opts.HasRelatedChanges()
		if tracked {
			result.Updates = make([]UpdateEntry, len(ids))
			for i, target := range ids {
				result.Updates[i] = UpdateEntry{Ticket: s.ByID[target], Publication: "pending"}
			}
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
			data, changed := current.Source, false
			var err error
			if c := changes[targetID]; c.HasChanges() {
				data, changed, err = ticket.PatchChanges(current, c)
			}
			if err != nil {
				result.Diagnostics = append(result.Diagnostics, operationError(err, current.Path, false))
				return
			}
			batch[i] = prepared{ticket: current, data: data, changed: changed}
			if !changed {
				if tracked {
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
				if tracked {
					result.Updates[i] = UpdateEntry{Ticket: item.ticket, Publication: "committed"}
				}
				// Advance only our expected file identity, never reload unrelated
				// inputs (which would accept an external edit between renames).
				s.Files[item.ticket.Path] = project.File{Data: item.data, Info: item.info}
				s.ByID[item.ticket.ID] = item.ticket
				s.InvalidateRelationships()
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

func checkRevision(s *project.Snapshot, id string, expected *string) []diagnostic.Diagnostic {
	if expected == nil {
		return nil
	}
	if !ticket.IDPattern.MatchString(id) || !slices.Contains(s.Names, id+".md") {
		return []diagnostic.Diagnostic{diagnostic.New("NOT_FOUND", "ticket no longer exists in this project; reload before retrying", "")}
	}
	path := ".wrk/" + id + ".md"
	file, readable := s.Files[path]
	if !readable || ticket.Revision(file.Data) != *expected {
		return []diagnostic.Diagnostic{diagnostic.New("CONFLICT", "ticket changed since it was read; reload and review before retrying", path)}
	}
	return nil
}
