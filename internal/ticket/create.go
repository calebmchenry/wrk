package ticket

import (
	"bytes"
	"fmt"
	"go.yaml.in/yaml/v3"
)

func Create(id, title, priority string, parent *string, labels []string, body []byte) ([]byte, error) {
	return CreateWithMetadata(id, title, priority, parent, labels, nil, nil, body)
}

func CreateWithMetadata(id, title, priority string, parent *string, labels, dependencies []string, fields []FieldValue, body []byte) ([]byte, error) {
	return CreateWithRelated(id, title, priority, parent, labels, dependencies, nil, fields, body)
}

func CreateWithRelated(id, title, priority string, parent *string, labels, dependencies, related []string, fields []FieldValue, body []byte) ([]byte, error) {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	scalar := func(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
	add := func(k string, v *yaml.Node) { n.Content = append(n.Content, scalar(k), v) }
	add("id", scalar(id))
	add("title", scalar(title))
	add("status", scalar("todo"))
	if parent != nil {
		add("parent", scalar(*parent))
	}
	if len(dependencies) > 0 {
		add("depends_on", stringListNode(dependencies))
	}
	if len(related) > 0 {
		add("related", stringListNode(related))
	}
	add("priority", scalar(priority))
	list := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, label := range labels {
		list.Content = append(list.Content, scalar(label))
	}
	add("labels", list)
	if len(fields) > 0 {
		if err := ValidateFields(fields, nil); err != nil {
			return nil, err
		}
		values, _ := patchFields(nil, fields, nil)
		add("fields", values)
	}
	data, err := encode(n, body)
	if err != nil {
		return nil, err
	}
	after, ds := Parse(data, "")
	if len(ds) > 0 || !Equal(n, after.Node) || !bytes.Equal(body, after.Body) {
		return nil, fmt.Errorf("PRESERVATION_UNSUPPORTED: creation did not preserve YAML values and body")
	}
	return data, nil
}
