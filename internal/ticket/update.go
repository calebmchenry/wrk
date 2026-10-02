package ticket

import (
	"bytes"
	"fmt"
	"go.yaml.in/yaml/v3"
	"slices"
	"unicode/utf8"
	"wrk/internal/schema"
)

func clone(n *yaml.Node, memo map[*yaml.Node]*yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if c := memo[n]; c != nil {
		return c
	}
	c := *n
	memo[n] = &c
	c.Content = nil
	for _, v := range n.Content {
		c.Content = append(c.Content, clone(v, memo))
	}
	c.Alias = clone(n.Alias, memo)
	return &c
}

// Equal compares the resolved graph, retaining tags, exact scalar precision, and
// cycles without expanding aliases. Presentation (comments/style/anchors) is not data.
func Equal(a, b *yaml.Node) bool {
	type pair struct{ a, b *yaml.Node }
	seen := map[pair]bool{}
	var equal func(*yaml.Node, *yaml.Node) bool
	equal = func(a, b *yaml.Node) bool {
		a, b = schema.Resolve(a), schema.Resolve(b)
		if a == nil || b == nil {
			return a == b
		}
		p := pair{a, b}
		if seen[p] {
			return true
		}
		seen[p] = true
		if a.Kind != b.Kind || a.Tag != b.Tag || a.Value != b.Value || len(a.Content) != len(b.Content) {
			return false
		}
		for i := range a.Content {
			if !equal(a.Content[i], b.Content[i]) {
				return false
			}
		}
		return true
	}
	return equal(a, b)
}

type Changes struct {
	Title, Status           *string
	AddLabels, RemoveLabels []string
}

func (c Changes) Validate() error {
	if c.Title == nil && c.Status == nil && len(c.AddLabels) == 0 && len(c.RemoveLabels) == 0 {
		return fmt.Errorf("update requires --title, --status, --add-label, or --remove-label")
	}
	for _, labels := range [][]string{c.AddLabels, c.RemoveLabels} {
		for _, label := range labels {
			if label == "" || !utf8.ValidString(label) {
				return fmt.Errorf("labels must be nonempty UTF-8 strings")
			}
		}
	}
	for _, label := range c.AddLabels {
		if slices.Contains(c.RemoveLabels, label) {
			return fmt.Errorf("cannot add and remove the same label %q", label)
		}
	}
	return nil
}

func stringNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

func Patch(t *Ticket, title, status *string) ([]byte, bool, error) {
	return PatchChanges(t, Changes{Title: title, Status: status})
}

func PatchChanges(t *Ticket, c Changes) ([]byte, bool, error) {
	if err := c.Validate(); err != nil {
		return nil, false, err
	}
	changes := map[string]*yaml.Node{}
	old := schema.Map(t.Node)
	for key, v := range map[string]*string{"title": c.Title, "status": c.Status} {
		if v != nil {
			s, _ := schema.String(old[key])
			if s != *v {
				changes[key] = stringNode(*v)
			}
		}
	}
	labels, _ := schema.StringList(old["labels"], true)
	next := []string{}
	for _, label := range labels {
		if !slices.Contains(c.RemoveLabels, label) {
			next = append(next, label)
		}
	}
	for _, label := range c.AddLabels {
		if !slices.Contains(next, label) {
			next = append(next, label)
		}
	}
	if !slices.Equal(labels, next) {
		node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, label := range next {
			node.Content = append(node.Content, stringNode(label))
		}
		changes["labels"] = node
	}
	if len(changes) == 0 {
		return bytes.Clone(t.Source), false, nil
	}
	n := clone(t.Node, map[*yaml.Node]*yaml.Node{})
	m := schema.Map(n)
	// Detach aliases of removed definitions, including a labels sequence and its
	// anchored elements. Built-in edited values resolve only to strings/lists of
	// strings, so copying them cannot expand arbitrary custom alias graphs.
	for _, key := range []string{"title", "status", "labels"} {
		if changes[key] == nil {
			continue
		}
		target := m[key]
		if target == nil {
			continue
		}
		removed := map[*yaml.Node]bool{target: true}
		for _, child := range target.Content {
			removed[child] = true
		}
		seen := map[*yaml.Node]bool{}
		var detach func(*yaml.Node)
		detach = func(v *yaml.Node) {
			if v == nil || seen[v] {
				return
			}
			seen[v] = true
			if v.Kind == yaml.AliasNode && removed[v.Alias] {
				resolved := schema.Resolve(v)
				if resolved != nil && resolved.Kind == yaml.ScalarNode {
					*v = *resolved
					v.Anchor = ""
					v.Alias = nil
				} else if values, ok := schema.StringList(resolved, true); ok {
					*v = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
					for _, value := range values {
						v.Content = append(v.Content, stringNode(value))
					}
				}
				return
			}
			for _, c := range v.Content {
				detach(c)
			}
		}
		detach(n)
	}
	inserted := 0
	for i := 0; i < len(n.Content); i += 2 {
		key, _ := schema.String(n.Content[i])
		if v, ok := changes[key]; ok {
			n.Content[i+1] = v
		}
	}
	if changes["labels"] != nil && old["labels"] == nil {
		n.Content = append(n.Content, stringNode("labels"), changes["labels"])
		inserted++
	}
	data, err := encode(n, t.Body)
	if err != nil {
		return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: %w", err)
	}
	after, ds := Parse(data, t.Path)
	if len(ds) > 0 {
		return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: emitted YAML did not reparse")
	}
	am := schema.Map(after.Node)
	if !bytes.Equal(t.Body, after.Body) || len(old)+inserted != len(am) {
		return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: body or field set changed")
	}
	for key, want := range changes {
		if !Equal(am[key], want) {
			return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: %s changed unexpectedly", key)
		}
	}
	for key, before := range old {
		if changes[key] == nil && !Equal(before, am[key]) {
			return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: unrelated %s changed", key)
		}
	}
	return data, true, nil
}
