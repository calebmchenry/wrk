package ticket

import (
	"go.yaml.in/yaml/v3"
	"regexp"
	"strings"
	"unicode/utf8"
	"wrk/internal/diagnostic"
	"wrk/internal/schema"
)

var IDPattern = regexp.MustCompile(`^[a-z][a-z0-9]{0,15}-[0-9a-f]{8}$`)
var FilenamePattern = regexp.MustCompile(`^[a-z][a-z0-9]{0,15}-[0-9a-f]{8}\.md$`)
var PrefixPattern = regexp.MustCompile(`^[a-z][a-z0-9]{0,15}$`)
var Statuses = map[string]bool{"todo": true, "in-progress": true, "done": true, "canceled": true}
var Priorities = map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}

type FieldDefinition struct {
	Type, Description string
	Options           []string
}

func ValidTitle(s string) bool {
	return utf8.ValidString(s) && strings.TrimSpace(s) != "" && !strings.ContainsAny(s, "\r\n\u0085\u2028\u2029")
}

func Validate(t *Ticket, definitions map[string]FieldDefinition) []diagnostic.Diagnostic {
	n := t.Node
	if n == nil {
		return nil
	}
	allowed := map[string]bool{"id": true, "title": true, "status": true, "parent": true, "depends_on": true, "priority": true, "labels": true, "fields": true}
	ds := schema.Keys(n, allowed, t.Path, "", "INVALID_TICKET")
	m := schema.Map(n)
	add := func(field, message string) {
		ds = append(ds, schema.At("INVALID_TICKET", message, t.Path, field, m[field]))
	}
	for _, key := range []string{"id", "title", "status"} {
		if _, ok := schema.String(m[key]); !ok {
			add(key, "required field must be a string")
		}
	}
	t.ID, _ = schema.String(m["id"])
	t.Title, _ = schema.String(m["title"])
	t.Status, _ = schema.String(m["status"])
	if s, ok := schema.String(m["id"]); ok && !IDPattern.MatchString(s) {
		add("id", "expected prefix and eight lowercase hexadecimal characters")
	}
	if s, ok := schema.String(m["title"]); ok && !ValidTitle(s) {
		add("title", "title must be nonblank and single-line")
	}
	if s, ok := schema.String(m["status"]); ok && !Statuses[s] {
		add("status", "expected todo, in-progress, done, or canceled")
	}
	if v, exists := m["parent"]; exists {
		s, ok := schema.String(v)
		if !ok || !IDPattern.MatchString(s) {
			add("parent", "expected a ticket ID string; omit parent for a root")
		} else {
			t.Parent = &s
		}
	}
	if v, exists := m["priority"]; exists {
		s, ok := schema.String(v)
		if !ok || !Priorities[s] {
			add("priority", "expected low, normal, high, or urgent")
		} else {
			t.Priority = s
		}
	}
	for _, key := range []string{"depends_on", "labels"} {
		if v, exists := m[key]; exists {
			ss, ok := schema.StringList(v, true)
			if !ok {
				add(key, "expected a list of nonempty strings")
				continue
			}
			if key == "labels" {
				t.Labels = ss
			} else {
				t.DependsOn = ss
				for _, s := range ss {
					if !IDPattern.MatchString(s) {
						add(key, "dependencies must contain ticket IDs")
						break
					}
				}
			}
		}
	}
	if v, exists := m["fields"]; exists {
		v = schema.Resolve(v)
		if !schema.Is(v, yaml.MappingNode, "!!map") {
			add("fields", "expected a mapping")
		} else {
			for i := 0; i < len(v.Content); i += 2 {
				name, ok := schema.String(v.Content[i])
				if !ok || name == "" {
					ds = append(ds, schema.At("INVALID_TICKET", "custom field names must be nonempty strings", t.Path, "fields", v.Content[i]))
					continue
				}
				def, configured := definitions[name]
				if !configured {
					continue
				}
				value := schema.Resolve(v.Content[i+1])
				valid := false
				switch def.Type {
				case "string":
					valid = schema.Is(value, yaml.ScalarNode, "!!str")
				case "number":
					valid = schema.Number(value)
				case "boolean":
					valid = schema.Boolean(value)
				case "enum":
					s, ok := schema.String(value)
					if ok {
						for _, option := range def.Options {
							if s == option {
								valid = true
							}
						}
					}
				}
				if !valid {
					ds = append(ds, schema.At("INVALID_TICKET", "custom value must match configured "+def.Type+" type without coercion", t.Path, "fields."+name, value))
				}
			}
		}
	}
	return ds
}
