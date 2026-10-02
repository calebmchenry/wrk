package ticket

import (
	"bytes"
	"fmt"
	"go.yaml.in/yaml/v3"
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

func Patch(t *Ticket, title, status *string) ([]byte, bool, error) {
	changes := map[string]string{}
	old := schema.Map(t.Node)
	for key, v := range map[string]*string{"title": title, "status": status} {
		if v != nil {
			s, _ := schema.String(old[key])
			if s != *v {
				changes[key] = *v
			}
		}
	}
	if len(changes) == 0 {
		return bytes.Clone(t.Source), false, nil
	}
	n := clone(t.Node, map[*yaml.Node]*yaml.Node{})
	m := schema.Map(n)
	// Detach aliases of a changed anchored scalar before replacing its definition.
	for key := range changes {
		target := m[key]
		if target == nil {
			return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: missing %s", key)
		}
		seen := map[*yaml.Node]bool{}
		var detach func(*yaml.Node)
		detach = func(v *yaml.Node) {
			if v == nil || seen[v] {
				return
			}
			seen[v] = true
			if v.Kind == yaml.AliasNode && v.Alias == target {
				resolved := schema.Resolve(target)
				if resolved != nil && resolved.Kind == yaml.ScalarNode {
					*v = *resolved
					v.Anchor = ""
					v.Alias = nil
				}
				return
			}
			for _, c := range v.Content {
				detach(c)
			}
		}
		detach(n)
	}
	for i := 0; i < len(n.Content); i += 2 {
		if v, ok := changes[n.Content[i].Value]; ok {
			n.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
		}
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
	if !bytes.Equal(t.Body, after.Body) || len(old) != len(am) {
		return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: body or field set changed")
	}
	for key, before := range old {
		if want, ok := changes[key]; ok {
			if got, valid := schema.String(am[key]); !valid || got != want {
				return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: %s changed unexpectedly", key)
			}
		} else if !Equal(before, am[key]) {
			return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: unrelated %s changed", key)
		}
	}
	return data, true, nil
}
