// Package schema provides shared strict YAML node checks without Go-value coercion.
package schema

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
	"wrk/internal/diagnostic"
)

func Parse(data []byte, path, code string) (*yaml.Node, []diagnostic.Diagnostic) {
	fail := func(s string) (*yaml.Node, []diagnostic.Diagnostic) {
		return nil, []diagnostic.Diagnostic{diagnostic.New(code, s, path)}
	}
	if !utf8.Valid(data) {
		return fail("source must be UTF-8")
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc, extra yaml.Node
	if err := dec.Decode(&doc); err != nil {
		return fail("cannot parse YAML: " + err.Error())
	}
	if err := dec.Decode(&extra); err != io.EOF {
		return fail("expected exactly one YAML document")
	}
	if len(doc.Content) != 1 {
		return fail("expected a YAML document")
	}
	ds := Duplicates(doc.Content[0], path, code)
	return doc.Content[0], ds
}

func Resolve(n *yaml.Node) *yaml.Node {
	seen := map[*yaml.Node]bool{}
	for n != nil && n.Kind == yaml.AliasNode {
		if seen[n] {
			return nil
		}
		seen[n] = true
		n = n.Alias
	}
	return n
}
func Is(n *yaml.Node, kind yaml.Kind, tag string) bool {
	n = Resolve(n)
	return n != nil && n.Kind == kind && n.Tag == tag
}
func String(n *yaml.Node) (string, bool) {
	n = Resolve(n)
	if n == nil || !Is(n, yaml.ScalarNode, "!!str") {
		return "", false
	}
	return n.Value, true
}
func Map(n *yaml.Node) map[string]*yaml.Node {
	n = Resolve(n)
	m := map[string]*yaml.Node{}
	if n == nil || n.Kind != yaml.MappingNode {
		return m
	}
	for i := 0; i < len(n.Content); i += 2 {
		if key, ok := String(n.Content[i]); ok {
			m[key] = n.Content[i+1]
		}
	}
	return m
}
func At(code, message, path, field string, n *yaml.Node) diagnostic.Diagnostic {
	d := diagnostic.Diagnostic{Code: code, Message: message, Path: path, Field: field}
	if n != nil {
		d.Line = n.Line
		d.Column = n.Column
	}
	return d
}

// Signatures retain exact scalar values and memoize graph nodes. Hashing each
// collection prevents alias-heavy complex keys from expanding exponentially.
func keySignature(root *yaml.Node) string {
	active := map[*yaml.Node]bool{}
	memo := map[*yaml.Node]string{}
	var signature func(*yaml.Node) string
	signature = func(n *yaml.Node) string {
		n = Resolve(n)
		if n == nil {
			return "nil"
		}
		if active[n] {
			return "recursive"
		}
		if s, ok := memo[n]; ok {
			return s
		}
		active[n] = true
		defer delete(active, n)
		value := n.Value
		if n.Kind == yaml.ScalarNode {
			switch n.Tag {
			case "!!int":
				if v, ok := integer(value); ok {
					value = v.String()
				}
			case "!!bool":
				value = strings.ToLower(value)
			case "!!null":
				value = ""
			}
		}
		parts := []string{}
		if n.Kind == yaml.MappingNode {
			for i := 0; i < len(n.Content); i += 2 {
				parts = append(parts, signature(n.Content[i])+":"+signature(n.Content[i+1]))
			}
			sort.Strings(parts)
		} else {
			for _, c := range n.Content {
				parts = append(parts, signature(c))
			}
		}
		digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%q:%q:[%s]", n.Kind, n.Tag, value, strings.Join(parts, ","))))
		sig := fmt.Sprintf("%x", digest)
		memo[n] = sig
		return sig
	}
	return signature(root)
}

func integer(value string) (*big.Int, bool) {
	value = strings.ReplaceAll(value, "_", "")
	return new(big.Int).SetString(value, 0)
}

var floatSyntax = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

func Number(n *yaml.Node) bool {
	n = Resolve(n)
	if n == nil || n.Kind != yaml.ScalarNode {
		return false
	}
	if n.Tag == "!!int" {
		_, ok := integer(n.Value)
		return ok
	}
	if n.Tag != "!!float" {
		return false
	}
	s := strings.ToLower(strings.ReplaceAll(n.Value, "_", ""))
	return floatSyntax.MatchString(s) || s == ".inf" || s == "+.inf" || s == "-.inf" || s == ".nan"
}
func Boolean(n *yaml.Node) bool {
	n = Resolve(n)
	if !Is(n, yaml.ScalarNode, "!!bool") {
		return false
	}
	s := strings.ToLower(n.Value)
	return s == "true" || s == "false"
}
func Duplicates(root *yaml.Node, path, code string) []diagnostic.Diagnostic {
	ds := []diagnostic.Diagnostic{}
	seen := map[*yaml.Node]bool{}
	var walk func(*yaml.Node, string)
	walk = func(n *yaml.Node, field string) {
		n = Resolve(n)
		if n == nil || seen[n] {
			return
		}
		seen[n] = true
		if n.Kind == yaml.MappingNode {
			keys := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				sig := keySignature(k)
				child := field
				if s, ok := String(k); ok {
					if child != "" {
						child += "."
					}
					child += s
				}
				if keys[sig] {
					ds = append(ds, At(code, "duplicate YAML key", path, child, k))
				}
				keys[sig] = true
				walk(k, child)
				walk(v, child)
			}
		} else {
			for _, c := range n.Content {
				walk(c, field)
			}
		}
	}
	walk(root, "")
	return ds
}

func Keys(n *yaml.Node, allowed map[string]bool, path, field, code string) []diagnostic.Diagnostic {
	n = Resolve(n)
	ds := []diagnostic.Diagnostic{}
	if !Is(n, yaml.MappingNode, "!!map") {
		return append(ds, At(code, "expected a mapping", path, field, n))
	}
	for i := 0; i < len(n.Content); i += 2 {
		key, ok := String(n.Content[i])
		f := key
		if field != "" {
			f = field + "." + key
		}
		if !ok || !allowed[key] {
			ds = append(ds, At(code, "unknown or non-string setting", path, f, n.Content[i]))
		}
	}
	return ds
}
func StringList(n *yaml.Node, nonempty bool) ([]string, bool) {
	n = Resolve(n)
	out := []string{}
	if !Is(n, yaml.SequenceNode, "!!seq") {
		return out, false
	}
	for _, c := range n.Content {
		s, ok := String(c)
		if !ok || (nonempty && s == "") {
			return out, false
		}
		out = append(out, s)
	}
	return out, true
}
