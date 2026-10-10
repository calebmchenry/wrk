package ticket

import (
	"bytes"
	"fmt"
	"go.yaml.in/yaml/v3"
	"unicode/utf8"
	"wrk/internal/diagnostic"
	"wrk/internal/schema"
)

type Ticket struct {
	Source, Body                []byte
	Node                        *yaml.Node
	ID, Title, Status, Priority string
	Parent                      *string
	DependsOn, Labels, Related  []string
	Path                        string
}

func Parse(data []byte, path string) (*Ticket, []diagnostic.Diagnostic) {
	t := &Ticket{Source: bytes.Clone(data), Path: path, DependsOn: []string{}, Labels: []string{}, Priority: "normal"}
	fail := func(s string) (*Ticket, []diagnostic.Diagnostic) {
		return t, []diagnostic.Diagnostic{diagnostic.New("INVALID_TICKET", s, path)}
	}
	if !utf8.Valid(data) {
		return fail("ticket must be UTF-8")
	}
	line := func(start int) ([]byte, int) {
		end := bytes.IndexByte(data[start:], '\n')
		if end < 0 {
			return data[start:], len(data)
		}
		end += start
		b := data[start:end]
		b = bytes.TrimSuffix(b, []byte{'\r'})
		return b, end + 1
	}
	first, start := line(0)
	if !bytes.Equal(first, []byte("---")) || start == len(data) {
		return fail("expected opening --- line and frontmatter")
	}
	end := -1
	bodyStart := 0
	for p := start; p < len(data); {
		b, next := line(p)
		if bytes.Equal(b, []byte("---")) {
			end = p
			bodyStart = next
			break
		}
		p = next
	}
	if end < 0 {
		return fail("missing closing --- delimiter")
	}
	t.Body = bytes.Clone(data[bodyStart:])
	n, ds := schema.Parse(data[start:end], path, "INVALID_TICKET")
	t.Node = n
	// YAML locations are relative to the frontmatter; account for its opener.
	seen := map[*yaml.Node]bool{}
	var shift func(*yaml.Node)
	shift = func(n *yaml.Node) {
		if n == nil || seen[n] {
			return
		}
		seen[n] = true
		n.Line++
		for _, c := range n.Content {
			shift(c)
		}
		shift(n.Alias)
	}
	shift(n)
	for i := range ds {
		if ds[i].Line > 0 {
			ds[i].Line++
		}
	}
	if n != nil && !schema.Is(n, yaml.MappingNode, "!!map") {
		ds = append(ds, diagnostic.New("INVALID_TICKET", "frontmatter must be a mapping", path))
	}
	if n != nil {
		m := schema.Map(n)
		t.ID, _ = schema.String(m["id"])
	}
	return t, ds
}

func encode(node *yaml.Node, body []byte) ([]byte, error) {
	node, err := emissionGraph(node)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString("---\n")
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, fmt.Errorf("encode frontmatter: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	b.WriteString("---\n")
	b.Write(body)
	return b.Bytes(), nil
}
