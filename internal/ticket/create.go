package ticket

import "go.yaml.in/yaml/v3"

func Create(id, title, priority string, parent *string, labels []string, body []byte) ([]byte, error) {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	scalar := func(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
	add := func(k string, v *yaml.Node) { n.Content = append(n.Content, scalar(k), v) }
	add("id", scalar(id))
	add("title", scalar(title))
	add("status", scalar("todo"))
	if parent != nil {
		add("parent", scalar(*parent))
	}
	add("priority", scalar(priority))
	list := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, label := range labels {
		list.Content = append(list.Content, scalar(label))
	}
	add("labels", list)
	return encode(n, body)
}
