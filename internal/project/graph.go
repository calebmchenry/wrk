package project

import (
	"slices"
	"sort"
	"strings"
	"wrk/internal/diagnostic"
	"wrk/internal/ticket"
)

type Blocker struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
type Summary struct {
	ID              string    `json:"id"`
	Path            string    `json:"path"`
	Revision        string    `json:"revision"`
	Title           string    `json:"title"`
	Status          string    `json:"status"`
	Parent          *string   `json:"parent"`
	DependsOn       []string  `json:"depends_on"`
	Related         []string  `json:"related"`
	RelatedRevision string    `json:"related_revision"`
	Priority        string    `json:"priority"`
	Labels          []string  `json:"labels"`
	Blockers        []Blocker `json:"blockers"`
}

func (s *Snapshot) Summary(t *ticket.Ticket) Summary {
	deps := append([]string{}, t.DependsOn...)
	sort.Strings(deps)
	labels := append([]string{}, t.Labels...)
	sort.Strings(labels)
	return Summary{
		ID: t.ID, Path: t.Path, Revision: ticket.Revision(t.Source), Title: t.Title,
		Status: t.Status, Parent: t.Parent, DependsOn: deps, Related: s.RelatedIDs(t.ID),
		RelatedRevision: s.RelatedRevision(t.ID), Priority: t.Priority, Labels: labels, Blockers: s.Blockers(t),
	}
}

// InvalidateRelationships must follow changes to ByID in a mutable write snapshot.
func (s *Snapshot) InvalidateRelationships() { s.related = nil }

// RelatedIDs derives the symmetric view without storing reciprocal copies.
func (s *Snapshot) RelatedIDs(id string) []string {
	if s.related == nil {
		sets := map[string]map[string]bool{}
		add := func(a, b string) {
			if sets[a] == nil {
				sets[a] = map[string]bool{}
			}
			sets[a][b] = true
		}
		for _, t := range s.ByID {
			for _, other := range t.Related {
				add(t.ID, other)
				add(other, t.ID)
			}
		}
		s.related = map[string][]string{}
		for id, values := range sets {
			for value := range values {
				s.related[id] = append(s.related[id], value)
			}
			sort.Strings(s.related[id])
		}
	}
	return append([]string{}, s.related[id]...)
}

func (s *Snapshot) RelatedRevision(id string) string {
	return ticket.Revision([]byte(id + "\n" + strings.Join(s.RelatedIDs(id), "\n")))
}

func (s *Snapshot) Blockers(t *ticket.Ticket) []Blocker {
	out := []Blocker{}
	for _, id := range t.DependsOn {
		if dep := s.ByID[id]; dep != nil && dep.Status != "done" {
			out = append(out, Blocker{id, dep.Status})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (s *Snapshot) Children(id string) []Summary {
	out := []Summary{}
	for _, t := range s.Tickets {
		if t.Parent != nil && *t.Parent == id {
			out = append(out, s.Summary(t))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (s *Snapshot) List(all, ready bool) []Summary {
	return s.ScopedList(all, ready, nil, nil)
}

// Descendants excludes the root and follows parent edges only.
func (s *Snapshot) Descendants(id string) map[string]bool {
	children := map[string][]string{}
	for _, t := range s.Tickets {
		if t.Parent != nil {
			children[*t.Parent] = append(children[*t.Parent], t.ID)
		}
	}
	out := map[string]bool{}
	queue := append([]string{}, children[id]...)
	for len(queue) > 0 {
		next := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if out[next] {
			continue
		}
		out[next] = true
		queue = append(queue, children[next]...)
	}
	return out
}

// ScopedList intersects all labels and descendants with the status filter.
// The caller validates that an explicitly supplied root exists.
func (s *Snapshot) ScopedList(all, ready bool, labels []string, under *string) []Summary {
	var descendants map[string]bool
	if under != nil {
		descendants = s.Descendants(*under)
	}
	out := []Summary{}
	for _, t := range s.Tickets {
		if under != nil && !descendants[t.ID] {
			continue
		}
		matches := true
		for _, label := range labels {
			if !slices.Contains(t.Labels, label) {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		if ready {
			if t.Status != "todo" || len(s.Blockers(t)) > 0 {
				continue
			}
		} else if !all && t.Status != "todo" && t.Status != "in-progress" && t.Status != "blocked" {
			continue
		}
		out = append(out, s.Summary(t))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func ValidateGraphs(s *Snapshot) []diagnostic.Diagnostic {
	ds := []diagnostic.Diagnostic{}
	ids := []string{}
	for id := range s.ByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, field := range []string{"parent", "depends_on", "related"} {
		edges := map[string][]string{}
		relatedPairs := map[[2]string]bool{}
		for _, id := range ids {
			t := s.ByID[id]
			refs := []string{}
			if field == "parent" {
				if t.Parent != nil {
					refs = append(refs, *t.Parent)
				}
			} else if field == "related" {
				refs = append(refs, t.Related...)
			} else {
				refs = append(refs, t.DependsOn...)
			}
			seen := map[string]bool{}
			for _, ref := range refs {
				code, message := "", ""
				if seen[ref] {
					code, message = "DUPLICATE_DEPENDENCY", "dependency is listed more than once"
				} else if ref == id {
					code, message = "SELF_REFERENCE", "ticket cannot reference itself"
				} else if s.ByID[ref] == nil {
					code, message = "MISSING_REFERENCE", "referenced ticket does not exist"
				}
				if field == "related" {
					if seen[ref] {
						code, message = "DUPLICATE_RELATED", "related item is listed more than once"
					}
					if code == "" {
						pair := [2]string{id, ref}
						if id > ref {
							pair = [2]string{ref, id}
						}
						if relatedPairs[pair] {
							code, message = "DUPLICATE_RELATED", "related pair must be stored on only one endpoint"
						}
						relatedPairs[pair] = true
					}
				}
				seen[ref] = true
				if code != "" {
					ds = append(ds, diagnostic.Diagnostic{Code: code, Message: message, Path: t.Path, Field: field, IDs: []string{id, ref}})
				} else {
					edges[id] = append(edges[id], ref)
				}
			}
			sort.Strings(edges[id])
		}
		if field == "related" {
			continue // Contextual links may form cycles.
		}
		color := map[string]int{}
		stack := []string{}
		positions := map[string]int{}
		var visit func(string)
		visit = func(id string) {
			color[id] = 1
			positions[id] = len(stack)
			stack = append(stack, id)
			for _, next := range edges[id] {
				if color[next] == 0 {
					visit(next)
				} else if color[next] == 1 {
					cycle := append([]string{}, stack[positions[next]:]...)
					sort.Strings(cycle)
					ds = append(ds, diagnostic.Diagnostic{Code: "CYCLE", Message: "cycle in " + field + " graph", Path: s.ByID[id].Path, Field: field, IDs: cycle})
				}
			}
			stack = stack[:len(stack)-1]
			color[id] = 2
		}
		for _, id := range ids {
			if color[id] == 0 {
				visit(id)
			}
		}
	}
	diagnostic.Sort(ds)
	return ds
}
