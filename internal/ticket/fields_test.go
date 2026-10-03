package ticket

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"wrk/internal/schema"
)

func fieldValues(t *testing.T, values ...string) []FieldValue {
	t.Helper()
	fields := []FieldValue{}
	for _, value := range values {
		field, err := ParseField(value)
		if err != nil {
			t.Fatal(err)
		}
		fields = append(fields, field)
	}
	return fields
}

func TestFieldInputYAML(t *testing.T) {
	for _, source := range []string{
		"x=", "x=  ", "x=# comment", "x=---\n", "=value", "no-equals", "x=[", "x=*missing", "\xff=value", "x=\xff",
		"x=one\n---\ntwo", "x={nested: {a: 1, a: 2}}", "x={1: a, 01: b}", "bad\\name=value", "trailing\\",
	} {
		if _, err := ParseField(source); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
	for _, source := range []string{
		"x=null", "x=''", "x='false'", "x=false", "x=1.23456789012345678901234567890",
		"x=!!int 1234567890123456789012345678901234567890", "x=!custom {data: [a, b]}",
		"x=&loop [*loop]", "x=a=b", "dots.are.literal=hello", "x=|\n  ---\n  body\n",
		"equals\\=and\\\\slash='\\n literal'", "spaces and 🦊=value",
	} {
		fields := fieldValues(t, source)
		data, err := CreateWithMetadata("wrk-12345678", "Title", "normal", nil, nil, nil, fields, []byte("\r\nbody"))
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		created, ds := Parse(data, "fixture")
		if len(ds) > 0 || !Equal(schema.Map(schema.Map(created.Node)["fields"])[fields[0].Name], fields[0].Value) {
			t.Fatalf("lost value %q: %v", source, ds)
		}
	}
}

func TestCustomFieldPatchesPreserveGraphs(t *testing.T) {
	for _, front := range []string{
		"fields: {edit: &old !opaque {huge: !!int 123456789012345678901234567890, loop: &loop [*loop]}, keep: *old}\n",
		"fields: &fields {edit: old, keep: *fields}\n",
		"fields: {edit: &oldKey old, ? *oldKey : {data: !tag value}}\n",
		"fields: {edit: &oldValue todo}\nstatus: *oldValue\n",
		"fields: {edit: &oldValue wrk-87654321}\nparent: *oldValue\n",
	} {
		for _, remove := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/remove=%t", front, remove), func(t *testing.T) {
				prefix := "---\nid: wrk-12345678\ntitle: Title\n"
				if !strings.Contains(front, "status:") {
					prefix += "status: todo\n"
				}
				source := []byte(prefix + front + "---\r\nBody 🦊\r\n---\nno newline")
				before, ds := Parse(source, "fixture")
				if len(ds) > 0 {
					t.Fatal(ds)
				}
				changes := Changes{Fields: fieldValues(t, "new=&old {a: &loop [*loop], b: *loop}", "second=&old !tag [*old]")}
				if remove {
					changes.RemoveFields = []string{"edit", "edit", "absent"}
				} else {
					changes.Fields = append(changes.Fields, fieldValues(t, "edit=!!float 1.23456789012345678901234567890")...)
				}
				data, changed, err := PatchChanges(before, changes)
				if err != nil || !changed {
					t.Fatalf("%t %v", changed, err)
				}
				after, ds := Parse(data, "fixture")
				if len(ds) > 0 || len(Validate(after, nil)) > 0 || !bytes.Equal(before.Body, after.Body) {
					t.Fatalf("invalid result: %s %v", data, ds)
				}
				bm, am := schema.Map(before.Node), schema.Map(after.Node)
				for key, value := range bm {
					if key != "fields" && !Equal(value, am[key]) {
						t.Fatal("changed built-in", key)
					}
				}
				bf, af := schema.Map(bm["fields"]), schema.Map(am["fields"])
				for name, value := range bf {
					if name != "edit" && !Equal(value, af[name]) {
						t.Fatal("changed unrelated field", name)
					}
				}
				if remove && af["edit"] != nil {
					t.Fatal("did not remove field")
				}
				for _, field := range changes.Fields {
					if !Equal(field.Value, af[field.Name]) {
						t.Fatal("changed intended value", field.Name)
					}
				}
				if !bytes.Equal(before.Source, source) {
					t.Fatal("mutated original input")
				}
				again, changed, err := PatchChanges(after, changes)
				if err != nil || changed || !bytes.Equal(data, again) {
					t.Fatal("retry not a no-op", err)
				}
			})
		}
	}
}

func TestCustomFieldOmissionsAndUnsupportedPreservation(t *testing.T) {
	for _, front := range []string{"", "fields: {}\n", "fields: {keep: &loop [*loop]}\n"} {
		source := []byte("---\nid: wrk-12345678\ntitle: Title\nstatus: todo\n" + front + "---\nbody")
		before, _ := Parse(source, "fixture")
		data, changed, err := PatchChanges(before, Changes{RemoveFields: []string{"absent"}, NoParent: true})
		if err != nil || changed || !bytes.Equal(data, source) {
			t.Fatal("no-op removal changed source", err)
		}
	}
	source := []byte("---\n&whole\nid: wrk-12345678\ntitle: Title\nstatus: todo\nfields: {whole: *whole, edit: old}\n---\nbody")
	before, _ := Parse(source, "fixture")
	_, _, err := PatchChanges(before, Changes{Fields: fieldValues(t, "edit=new")})
	if err == nil || !strings.Contains(err.Error(), "PRESERVATION_UNSUPPORTED") || !bytes.Equal(before.Source, source) {
		t.Fatal("whole-ticket alias was not rejected unchanged", err)
	}
	// A no-op assignment cannot exempt a root alias from preservation checks
	// when another custom field changes in the same command.
	whole := schema.Map(schema.Map(before.Node)["fields"])["whole"]
	_, _, err = PatchChanges(before, Changes{Fields: append(fieldValues(t, "edit=new"), FieldValue{Name: "whole", Value: whole})})
	if err == nil || !strings.Contains(err.Error(), "PRESERVATION_UNSUPPORTED") {
		t.Fatal("no-op field assignment bypassed preservation", err)
	}
}

func TestRelationshipPatchesPreserveAnchors(t *testing.T) {
	source := []byte("---\nid: wrk-12345678\ntitle: Title\nstatus: todo\nparent: &parent wrk-87654321\ndepends_on: &deps [&dep wrk-87654321, wrk-aaaaaaaa]\nfields: {parent: *parent, deps: *deps, dep: *dep}\n---\r\nExact body")
	before, _ := Parse(source, "fixture")
	parent := "wrk-bbbbbbbb"
	for _, changes := range []Changes{
		{Parent: &parent, AddDependencies: []string{parent, parent}, RemoveDependencies: []string{"wrk-87654321"}},
		{NoParent: true, RemoveDependencies: []string{"wrk-87654321", "wrk-aaaaaaaa"}},
	} {
		data, changed, err := PatchChanges(before, changes)
		if err != nil || !changed {
			t.Fatal(err)
		}
		after, ds := Parse(data, "fixture")
		if len(ds) > 0 || !bytes.Equal(before.Body, after.Body) || !Equal(schema.Map(before.Node)["fields"], schema.Map(after.Node)["fields"]) {
			t.Fatal("relationship patch lost unrelated data", ds)
		}
		if changes.NoParent && schema.Map(after.Node)["parent"] != nil {
			t.Fatal("parent not omitted")
		}
		again, changed, err := PatchChanges(after, changes)
		if err != nil || changed || !bytes.Equal(again, data) {
			t.Fatal("retry not a no-op", err)
		}
	}
}
