package schema

import (
	"fmt"
	"go.yaml.in/yaml/v3"
	"strings"
	"testing"
)

func TestAliasHeavyKeysDoNotExpand(t *testing.T) {
	var s strings.Builder
	s.WriteString("base: &n0 [value]\n")
	for i := 1; i < 45; i++ {
		fmt.Fprintf(&s, "v%d: &n%d [*n%d, *n%d]\n", i, i, i-1, i-1)
	}
	s.WriteString("complex: { ? *n44 : value }\n")
	if _, ds := Parse([]byte(s.String()), "fixture", "BAD"); len(ds) > 0 {
		t.Fatal(ds)
	}
}
func TestNestedAndResolvedDuplicates(t *testing.T) {
	for _, text := range []string{"a: {b: 1, b: 2}", "a: {1: one, 0x1: two}", "a: {true: one, TRUE: two}", "a: { ? [a, b] : one, ? [a, b] : two }"} {
		if _, ds := Parse([]byte(text), "fixture", "BAD"); len(ds) == 0 {
			t.Fatalf("missed duplicate %s", text)
		}
	}
}
func TestStrictTypedScalars(t *testing.T) {
	for _, tc := range []struct {
		tag, value string
		valid      bool
	}{
		{"!!int", "1234567890123456789012345678901234567890", true}, {"!!int", "0x123456789abcdef123456789", true}, {"!!int", "oops", false}, {"!!float", "1.234567890123456789e+999999", true}, {"!!float", ".inf", true}, {"!!float", "oops", false}, {"!!bool", "yes", false}, {"!!bool", "True", true},
	} {
		n := &yaml.Node{Kind: yaml.ScalarNode, Tag: tc.tag, Value: tc.value}
		valid := Number(n)
		if tc.tag == "!!bool" {
			valid = Boolean(n)
		}
		if valid != tc.valid {
			t.Fatal(tc)
		}
	}
}
