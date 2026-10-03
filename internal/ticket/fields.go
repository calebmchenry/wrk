package ticket

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
	"wrk/internal/schema"
)

// FieldValue holds a YAML graph, not a Go value: decoding through interface{}
// would lose tags, alias structure, and exact numeric representations.
type FieldValue struct {
	Name  string
	Value *yaml.Node
}

func ParseField(value string) (FieldValue, error) {
	name, source, found := splitFieldAssignment(value)
	if !found || name == "" || !utf8.ValidString(name) || strings.TrimSpace(source) == "" {
		return FieldValue{}, fmt.Errorf("--field requires name=YAML with a nonempty UTF-8 name and value")
	}
	n, ds := schema.Parse([]byte(source), "", "USAGE")
	if len(ds) > 0 {
		return FieldValue{}, fmt.Errorf("--field %q: %s", name, ds[0].Message)
	}
	if n.Kind == yaml.ScalarNode && n.Tag == "!!null" && n.Value == "" {
		return FieldValue{}, fmt.Errorf("--field %q: empty YAML value; use null explicitly", name)
	}
	return FieldValue{Name: name, Value: n}, nil
}

// Escape only the name, so YAML escapes on the right side remain untouched.
// This permits every valid field name, including literal '=' and '\' characters.
func splitFieldAssignment(value string) (string, string, bool) {
	var name strings.Builder
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '=':
			return name.String(), value[i+1:], true
		case '\\':
			i++
			if i == len(value) || (value[i] != '\\' && value[i] != '=') {
				return "", "", false
			}
		}
		name.WriteByte(value[i])
	}
	return "", "", false
}

func ValidateFields(fields []FieldValue, remove []string) error {
	seen := map[string]bool{}
	for _, field := range fields {
		if field.Name == "" || !utf8.ValidString(field.Name) || field.Value == nil {
			return fmt.Errorf("custom fields require nonempty UTF-8 names and YAML values")
		}
		if seen[field.Name] {
			return fmt.Errorf("custom field %q may only be set once", field.Name)
		}
		seen[field.Name] = true
		if ds := schema.Duplicates(field.Value, "", "USAGE"); len(ds) > 0 {
			return fmt.Errorf("custom field %q: %s", field.Name, ds[0].Message)
		}
	}
	for _, name := range remove {
		if name == "" || !utf8.ValidString(name) {
			return fmt.Errorf("--remove-field requires a nonempty UTF-8 name")
		}
		if seen[name] {
			return fmt.Errorf("cannot set and remove the same custom field %q", name)
		}
	}
	return nil
}

func patchFields(old *yaml.Node, fields []FieldValue, remove []string) (*yaml.Node, bool) {
	m := schema.Map(old)
	next := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if old != nil {
		// Copy the mapping edges, not the old mapping itself. An alias of the
		// fields map (including a recursive alias) must keep its old meaning.
		next.Content = append(next.Content, schema.Resolve(old).Content...)
	}
	changed := false
	for _, name := range remove {
		if m[name] != nil {
			setMapEntry(next, name, nil)
			changed = true
		}
	}
	for _, field := range fields {
		if !Equal(m[field.Name], field.Value) {
			setMapEntry(next, field.Name, clone(field.Value, map[*yaml.Node]*yaml.Node{}))
			changed = true
		}
	}
	return next, changed
}
