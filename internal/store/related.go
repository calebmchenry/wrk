package store

import (
	"slices"
	"wrk/internal/diagnostic"
	"wrk/internal/project"
	"wrk/internal/ticket"
)

// A new edge belongs to the addressed item. Existing edges keep their owner;
// removal from either endpoint patches only that owner's list. Every candidate
// remains valid independently, including each prefix of a partial publication.
func relatedChanges(s *project.Snapshot, id string, c ticket.Changes) (map[string]ticket.Changes, []diagnostic.Diagnostic) {
	target := s.ByID[id]
	for _, ref := range append(append([]string{}, c.AddRelated...), c.RemoveRelated...) {
		code, message := "", ""
		if ref == id {
			code, message = "SELF_REFERENCE", "ticket cannot reference itself"
		} else if s.ByID[ref] == nil {
			code, message = "MISSING_REFERENCE", "related ticket does not exist in this project"
		}
		if code != "" {
			return nil, []diagnostic.Diagnostic{{Code: code, Message: message, Path: target.Path, Field: "related", IDs: []string{id, ref}}}
		}
	}
	linked := s.RelatedIDs(id)
	add := []string{}
	for _, ref := range c.AddRelated {
		if !slices.Contains(linked, ref) {
			add = append(add, ref)
		}
	}
	c.AddRelated = add
	remove := c.RemoveRelated
	if c.NoRelated {
		remove = linked
	}
	changes := map[string]ticket.Changes{id: c}
	for _, ref := range remove {
		if owner := s.ByID[ref]; owner != nil && slices.Contains(owner.Related, id) {
			changes[ref] = ticket.Changes{RemoveRelated: []string{id}}
		}
	}
	return changes, nil
}
