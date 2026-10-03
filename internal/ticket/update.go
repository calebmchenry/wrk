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
	Title, Status, Priority             *string
	AddLabels, RemoveLabels             []string
	Labels                              []string
	LabelsSet                           bool
	Parent                              *string
	NoParent                            bool
	AddDependencies, RemoveDependencies []string
	Fields                              []FieldValue
	RemoveFields                        []string
}

func (c Changes) Validate() error {
	if !c.HasNonLabelChanges() && !c.LabelsSet && len(c.AddLabels) == 0 && len(c.RemoveLabels) == 0 {
		return fmt.Errorf("update requires at least one metadata change (see wrk help update)")
	}
	if c.Parent != nil && c.NoParent {
		return fmt.Errorf("--parent and --no-parent conflict")
	}
	for _, id := range c.AddDependencies {
		if slices.Contains(c.RemoveDependencies, id) {
			return fmt.Errorf("cannot add and remove the same dependency %q", id)
		}
	}
	for _, id := range c.RemoveDependencies {
		if !IDPattern.MatchString(id) {
			return fmt.Errorf("--remove-dependency requires a ticket ID, got %q", id)
		}
	}
	if err := ValidateFields(c.Fields, c.RemoveFields); err != nil {
		return err
	}
	if c.LabelsSet && (len(c.AddLabels) > 0 || len(c.RemoveLabels) > 0) {
		return fmt.Errorf("--label/--no-labels conflict with --add-label/--remove-label")
	}
	for _, labels := range [][]string{c.Labels, c.AddLabels, c.RemoveLabels} {
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

func (c Changes) HasNonLabelChanges() bool {
	return c.Title != nil || c.Status != nil || c.Priority != nil || c.Parent != nil || c.NoParent ||
		len(c.AddDependencies) > 0 || len(c.RemoveDependencies) > 0 || len(c.Fields) > 0 || len(c.RemoveFields) > 0
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
	for key, v := range map[string]*string{"title": c.Title, "status": c.Status, "priority": c.Priority} {
		if v != nil {
			s, _ := schema.String(old[key])
			if key == "priority" && old[key] == nil {
				s = "normal"
			}
			if s != *v {
				changes[key] = stringNode(*v)
			}
		}
	}
	labels, _ := schema.StringList(old["labels"], true)
	next := editList(labels, c.AddLabels, c.RemoveLabels)
	if c.LabelsSet {
		next = c.Labels
	}
	if !slices.Equal(labels, next) {
		changes["labels"] = stringListNode(next)
	}
	if c.Parent != nil {
		parent, _ := schema.String(old["parent"])
		if old["parent"] == nil || parent != *c.Parent {
			changes["parent"] = stringNode(*c.Parent)
		}
	} else if c.NoParent && old["parent"] != nil {
		changes["parent"] = nil // A cleared parent is omitted, never null.
	}
	dependencies, _ := schema.StringList(old["depends_on"], true)
	nextDependencies := editList(dependencies, c.AddDependencies, c.RemoveDependencies)
	if !slices.Equal(dependencies, nextDependencies) {
		changes["depends_on"] = stringListNode(nextDependencies)
	}

	n := clone(t.Node, map[*yaml.Node]*yaml.Node{})
	m := schema.Map(n)
	fields, fieldsChanged := patchFields(m["fields"], c.Fields, c.RemoveFields)
	if fieldsChanged {
		changes["fields"] = fields
	}
	if len(changes) == 0 {
		return bytes.Clone(t.Source), false, nil
	}
	for _, key := range []string{"title", "status", "priority", "labels", "parent", "depends_on", "fields"} {
		if v, changed := changes[key]; changed {
			setMapEntry(n, key, v)
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
	wantCount := len(old)
	for key, value := range changes {
		if old[key] == nil && value != nil {
			wantCount++
		} else if old[key] != nil && value == nil {
			wantCount--
		}
		if !Equal(am[key], value) {
			return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: %s changed unexpectedly", key)
		}
	}
	if !bytes.Equal(t.Body, after.Body) || wantCount != len(am) {
		return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: body or field set changed")
	}
	for key, before := range old {
		if _, changed := changes[key]; !changed && !Equal(before, am[key]) {
			return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: unrelated %s changed", key)
		}
	}
	// A custom value may alias the entire frontmatter. Verify untouched values
	// against the original graph as well as the intended patched fields mapping.
	if fieldsChanged {
		touched := map[string]bool{}
		af := schema.Map(am["fields"])
		for _, field := range c.Fields {
			if !Equal(field.Value, af[field.Name]) {
				return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: intended fields.%s changed", field.Name)
			}
			touched[field.Name] = true
		}
		for _, name := range c.RemoveFields {
			touched[name] = true
		}
		for name, before := range schema.Map(old["fields"]) {
			if !touched[name] && !Equal(before, af[name]) {
				return nil, false, fmt.Errorf("PRESERVATION_UNSUPPORTED: unrelated fields.%s changed", name)
			}
		}
	}
	return data, true, nil
}

func editList(old, add, remove []string) []string {
	next := []string{}
	for _, value := range old {
		if !slices.Contains(remove, value) {
			next = append(next, value)
		}
	}
	for _, value := range add {
		if !slices.Contains(next, value) {
			next = append(next, value)
		}
	}
	return next
}

func stringListNode(values []string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, value := range values {
		n.Content = append(n.Content, stringNode(value))
	}
	return n
}

// Replace edges, never the referenced value nodes: aliases elsewhere retain
// their original targets. encode relocates any definitions no longer in Content.
func setMapEntry(n *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i < len(n.Content); i += 2 {
		name, _ := schema.String(n.Content[i])
		if name != key {
			continue
		}
		if value == nil {
			n.Content = append(n.Content[:i], n.Content[i+2:]...)
		} else {
			n.Content[i+1] = value
		}
		return
	}
	if value != nil {
		n.Content = append(n.Content, stringNode(key), value)
	}
}
