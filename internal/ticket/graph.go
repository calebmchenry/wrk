package ticket

import (
	"fmt"

	"go.yaml.in/yaml/v3"
	"wrk/internal/schema"
)

// emissionGraph serializes the reachable value graph in traversal order. The
// first occurrence defines a value; later occurrences become aliases. This
// relocates definitions removed by a patch, handles recursive graphs without
// expansion, and prevents anchor collisions between independent --field inputs.
// Anchor names and frontmatter formatting are deliberately not preserved.
func emissionGraph(root *yaml.Node) (*yaml.Node, error) {
	seen := map[*yaml.Node]*yaml.Node{}
	anchors := 0
	var visit func(*yaml.Node) (*yaml.Node, error)
	visit = func(n *yaml.Node) (*yaml.Node, error) {
		n = schema.Resolve(n)
		if n == nil {
			return nil, fmt.Errorf("unresolved YAML alias")
		}
		if previous := seen[n]; previous != nil {
			if previous.Anchor == "" {
				anchors++
				previous.Anchor = fmt.Sprintf("wrk%d", anchors)
			}
			return &yaml.Node{Kind: yaml.AliasNode, Value: previous.Anchor, Alias: previous}, nil
		}
		out := *n
		out.Anchor, out.Alias, out.Content = "", nil, nil
		seen[n] = &out
		for _, child := range n.Content {
			value, err := visit(child)
			if err != nil {
				return nil, err
			}
			out.Content = append(out.Content, value)
		}
		return &out, nil
	}
	return visit(root)
}
