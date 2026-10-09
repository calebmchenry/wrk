package store

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"wrk/internal/diagnostic"
	"wrk/internal/project"
	"wrk/internal/ticket"
)

type CreateOptions struct {
	Title            string
	Body             []byte
	Parent, Priority *string
	Labels           []string
	LabelsSet        bool
	Dependencies     []string
	Fields           []ticket.FieldValue
}

func Create(root string, options CreateOptions) Mutation {
	return create(root, options, rand.Reader, nil)
}
func create(root string, options CreateOptions, random io.Reader, h *hooks) Mutation {
	if err := ticket.ValidateFields(options.Fields, nil); err != nil {
		return Mutation{Diagnostics: []diagnostic.Diagnostic{diagnostic.New("USAGE", err.Error(), "")}}
	}
	return withLock(root, nil, func(s *project.Snapshot) Mutation {
		result := Mutation{Snapshot: s, Diagnostics: []diagnostic.Diagnostic{}}
		for attempt := 0; attempt < 128; attempt++ {
			token := make([]byte, 4)
			if _, err := io.ReadFull(random, token); err != nil {
				result.Diagnostics = append(result.Diagnostics, operationError(err, ".wrk", false))
				return result
			}
			id := s.Config.Prefix + "-" + hex.EncodeToString(token)
			path := ".wrk/" + id + ".md"
			target := filepath.Join(root, filepath.FromSlash(path))
			if s.ByID[id] != nil {
				continue
			}
			if _, err := os.Lstat(target); err == nil {
				continue
			} else if !os.IsNotExist(err) {
				result.Diagnostics = append(result.Diagnostics, operationError(err, path, false))
				return result
			}
			priority := s.Config.Priority
			if options.Priority != nil {
				priority = *options.Priority
			}
			labels := s.Config.Labels
			if options.LabelsSet {
				labels = options.Labels
			}
			data, err := ticket.CreateWithMetadata(id, options.Title, priority, options.Parent, labels, options.Dependencies, options.Fields, options.Body)
			if err != nil {
				result.Diagnostics = append(result.Diagnostics, operationError(err, path, false))
				return result
			}
			t, ds := s.Candidate(path, data)
			if len(ds) > 0 {
				result.Diagnostics = ds
				return result
			}
			staged, err := stage(filepath.Join(root, ".wrk"), data, nil, h)
			if err != nil {
				result.Diagnostics = append(result.Diagnostics, operationError(err, path, false))
				return result
			}
			publication := publish(staged, target, false, s.Compare, h)
			if publication.collision {
				// An external creator may win the final link race. Refresh and revalidate
				// all inputs before another ID attempt; never adopt an invalid project.
				s = project.Load(root)
				result.Snapshot = s
				if len(s.Diagnostics) > 0 {
					result.Diagnostics = s.Diagnostics
					return result
				}
				continue
			}
			result.Committed = publication.Committed
			result.Created = publication.Committed
			result.Ticket = t
			if publication.Err != nil {
				result.Diagnostics = append(result.Diagnostics, operationError(publication.Err, path, result.Committed))
			}
			return result
		}
		result.Diagnostics = append(result.Diagnostics, diagnostic.New("ID_EXHAUSTED", "unable to allocate an unused ID after 128 attempts", ".wrk"))
		return result
	})
}
