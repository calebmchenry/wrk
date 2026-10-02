package project

import (
	"go.yaml.in/yaml/v3"
	"wrk/internal/diagnostic"
	"wrk/internal/schema"
	"wrk/internal/ticket"
)

const DefaultConfig = "version: 1\nprefix: wrk\ndefaults:\n  priority: normal\n  labels: []\nfields: {}\n"
const ConfigPath = ".wrk/config.yaml"

type Config struct {
	Prefix, Priority string
	Labels           []string
	Fields           map[string]ticket.FieldDefinition
}

func ParseConfig(data []byte) (*Config, []diagnostic.Diagnostic) {
	c := &Config{Priority: "normal", Labels: []string{}, Fields: map[string]ticket.FieldDefinition{}}
	n, ds := schema.Parse(data, ConfigPath, "INVALID_CONFIG")
	if n == nil {
		return c, ds
	}
	ds = append(ds, schema.Keys(n, map[string]bool{"version": true, "prefix": true, "defaults": true, "fields": true}, ConfigPath, "", "INVALID_CONFIG")...)
	m := schema.Map(n)
	add := func(field, message string, node *yaml.Node) {
		ds = append(ds, schema.At("INVALID_CONFIG", message, ConfigPath, field, node))
	}
	version := schema.Resolve(m["version"])
	var v int
	if !schema.Is(version, yaml.ScalarNode, "!!int") || version.Decode(&v) != nil || v != 1 {
		add("version", "required integer version must be 1", version)
	}
	prefix, ok := schema.String(m["prefix"])
	if !ok || !ticket.PrefixPattern.MatchString(prefix) {
		add("prefix", "required prefix must match [a-z][a-z0-9]{0,15}", m["prefix"])
	} else {
		c.Prefix = prefix
	}
	if node, exists := m["defaults"]; exists {
		ds = append(ds, schema.Keys(node, map[string]bool{"priority": true, "labels": true}, ConfigPath, "defaults", "INVALID_CONFIG")...)
		defaults := schema.Map(node)
		if p, exists := defaults["priority"]; exists {
			value, ok := schema.String(p)
			if !ok || !ticket.Priorities[value] {
				add("defaults.priority", "expected low, normal, high, or urgent", p)
			} else {
				c.Priority = value
			}
		}
		if labels, exists := defaults["labels"]; exists {
			value, ok := schema.StringList(labels, true)
			if !ok {
				add("defaults.labels", "expected a list of nonempty strings", labels)
			} else {
				c.Labels = value
			}
		}
	}
	if node, exists := m["fields"]; exists {
		node = schema.Resolve(node)
		if !schema.Is(node, yaml.MappingNode, "!!map") {
			add("fields", "expected a mapping", node)
		} else {
			for i := 0; i < len(node.Content); i += 2 {
				name, ok := schema.String(node.Content[i])
				value := node.Content[i+1]
				field := "fields." + name
				if !ok || name == "" {
					add("fields", "field names must be nonempty strings", node.Content[i])
					continue
				}
				ds = append(ds, schema.Keys(value, map[string]bool{"type": true, "description": true, "options": true}, ConfigPath, field, "INVALID_CONFIG")...)
				def := ticket.FieldDefinition{}
				props := schema.Map(value)
				typ, ok := schema.String(props["type"])
				if !ok || (typ != "string" && typ != "number" && typ != "boolean" && typ != "enum") {
					add(field+".type", "required type must be string, number, boolean, or enum", props["type"])
				} else {
					def.Type = typ
				}
				if desc, exists := props["description"]; exists {
					s, ok := schema.String(desc)
					if !ok {
						add(field+".description", "description must be a string", desc)
					} else {
						def.Description = s
					}
				}
				opts, hasOptions := props["options"]
				if typ == "enum" {
					values, ok := schema.StringList(opts, false)
					seen := map[string]bool{}
					if !ok || len(values) == 0 {
						add(field+".options", "enum requires a nonempty list of distinct strings", opts)
					} else {
						for _, s := range values {
							if seen[s] {
								add(field+".options", "enum options must be distinct", opts)
								break
							}
							seen[s] = true
						}
						def.Options = values
					}
				} else if hasOptions {
					add(field+".options", "options are only valid for enum definitions", opts)
				}
				c.Fields[name] = def
			}
		}
	}
	return c, ds
}
