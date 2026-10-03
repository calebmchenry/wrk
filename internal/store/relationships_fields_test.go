package store

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"wrk/internal/project"
	"wrk/internal/schema"
	"wrk/internal/ticket"
)

func customFields(t *testing.T, values ...string) []ticket.FieldValue {
	t.Helper()
	fields := []ticket.FieldValue{}
	for _, value := range values {
		field, err := ticket.ParseField(value)
		if err != nil {
			t.Fatal(err)
		}
		fields = append(fields, field)
	}
	return fields
}

func assertProjectUnchanged(t *testing.T, root string, before *project.Snapshot) {
	t.Helper()
	after := project.Load(root)
	if len(before.Files) != len(after.Files) {
		t.Fatal("inventory changed")
	}
	for path, file := range before.Files {
		got := after.Files[path]
		if !bytes.Equal(file.Data, got.Data) || got.Info == nil || !os.SameFile(file.Info, got.Info) || file.Info.Mode() != got.Info.Mode() {
			t.Fatal("changed input", path)
		}
	}
}

func TestRelationshipValidationAndStatusIndependence(t *testing.T) {
	root := testProject(t)
	parent := testID
	child := Create(root, CreateOptions{Title: "Child", Parent: &parent})
	if len(child.Diagnostics) > 0 {
		t.Fatal(child.Diagnostics)
	}
	childID := child.Ticket.ID
	childSource := bytes.Clone(child.Ticket.Source)
	done := "done"
	// Parent and dependency graphs are independent: a parent can depend on its child.
	opts := UpdateOptions{Changes: ticket.Changes{AddDependencies: []string{childID, childID}, Status: &done}}
	m := UpdateWithOptions(root, testID, opts)
	if len(m.Diagnostics) > 0 || !m.Changed || m.Ticket.Status != done || !slices.Equal(m.Ticket.DependsOn, []string{childID}) {
		t.Fatalf("%+v", m)
	}
	s := project.Load(root)
	blockers := s.Summary(s.ByID[testID]).Blockers
	if !bytes.Equal(s.ByID[childID].Source, childSource) || len(blockers) != 1 || blockers[0].ID != childID {
		t.Fatal("status cascaded or blocker lost")
	}
	before := project.Load(root)
	m = UpdateWithOptions(root, testID, opts)
	if len(m.Diagnostics) > 0 || m.Changed {
		t.Fatal("repeated edge was not a no-op", m.Diagnostics)
	}
	assertProjectUnchanged(t, root, before)
	missing := "wrk-deadbeef"
	for _, test := range []struct {
		id      string
		changes ticket.Changes
		code    string
	}{
		{testID, ticket.Changes{Parent: &childID}, "CYCLE"},
		{childID, ticket.Changes{AddDependencies: []string{testID}}, "CYCLE"},
		{testID, ticket.Changes{Parent: &parent}, "SELF_REFERENCE"},
		{testID, ticket.Changes{AddDependencies: []string{testID}}, "SELF_REFERENCE"},
		{childID, ticket.Changes{Parent: &missing}, "MISSING_REFERENCE"},
		{testID, ticket.Changes{AddDependencies: []string{missing}}, "MISSING_REFERENCE"},
	} {
		title := "Do not publish"
		test.changes.Title = &title
		m = UpdateWithOptions(root, test.id, UpdateOptions{Changes: test.changes})
		if m.Committed || len(m.Diagnostics) == 0 || m.Diagnostics[0].Code != test.code {
			t.Fatalf("%+v", m)
		}
		assertProjectUnchanged(t, root, before)
	}
	for _, deps := range [][]string{{missing}, {childID, childID}, {"bad-id"}} {
		m = Create(root, CreateOptions{Title: "Invalid", Dependencies: deps})
		if m.Committed || len(m.Diagnostics) == 0 {
			t.Fatalf("accepted %v", deps)
		}
		assertProjectUnchanged(t, root, before)
	}
	// Clearing a child parent removes the key, preserves all other tickets, and
	// immediately changes derived children without affecting dependency blockers.
	m = UpdateWithOptions(root, childID, UpdateOptions{Changes: ticket.Changes{NoParent: true}})
	if len(m.Diagnostics) > 0 || m.Ticket.Parent != nil || schema.Map(m.Ticket.Node)["parent"] != nil {
		t.Fatalf("%+v", m)
	}
	s = project.Load(root)
	if len(s.Children(testID)) != 0 || !bytes.Equal(s.ByID[testID].Source, before.ByID[testID].Source) {
		t.Fatal("clear cascaded")
	}
	if m = Update(root, childID, nil, &done); len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	todo := "todo"
	if m = Update(root, testID, nil, &todo); len(m.Diagnostics) > 0 {
		t.Fatal(m.Diagnostics)
	}
	s = project.Load(root)
	if len(s.List(false, true)) != 1 || s.List(false, true)[0].ID != testID {
		t.Fatal("reopening/readiness failed")
	}
	for _, status := range []string{"canceled", "blocked"} {
		if m = Update(root, childID, nil, &status); len(m.Diagnostics) > 0 {
			t.Fatal(m.Diagnostics)
		}
		s = project.Load(root)
		if len(s.List(false, true)) != 0 || s.ByID[testID].Status != "todo" {
			t.Fatal("blocker/readiness cascade")
		}
	}
	blocked := "blocked"
	m = UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{RemoveDependencies: []string{childID}, Status: &blocked}})
	if len(m.Diagnostics) > 0 || len(m.Ticket.DependsOn) != 0 || len(project.Load(root).List(false, true)) != 0 {
		t.Fatal("manual blocked changed")
	}
}

func TestTypedFieldCreationAndUpdates(t *testing.T) {
	root := testProject(t)
	config := "version: 1\nprefix: wrk\nfields:\n  customer: {type: string}\n  estimate: {type: number}\n  review: {type: boolean}\n  area: {type: enum, options: [cli, storage]}\n"
	if err := os.WriteFile(filepath.Join(root, project.ConfigPath), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	fields := customFields(t, "customer='true'", "estimate=!!int 1234567890123456789012345678901234567890", "review=true", "area=cli", "opaque=!tag {nested: &loop [*loop]}")
	parent := testID
	m := Create(root, CreateOptions{Title: "Typed", Parent: &parent, Dependencies: []string{testID}, Fields: fields, Body: []byte("\r\nExact 🦊\r\nno newline")})
	if len(m.Diagnostics) > 0 || !m.Created {
		t.Fatal(m.Diagnostics)
	}
	id := m.Ticket.ID
	for _, field := range fields {
		if !ticket.Equal(field.Value, schema.Map(schema.Map(m.Ticket.Node)["fields"])[field.Name]) {
			t.Fatal("lost type/precision", field.Name)
		}
	}
	before := project.Load(root)
	for _, value := range []string{"customer=true", "estimate='12'", "review='true'", "review=1", "area=other", "area=null", "estimate=null", "estimate=!!int invalid"} {
		opts := CreateOptions{Title: "Invalid", Fields: customFields(t, value)}
		m = Create(root, opts)
		if m.Committed || len(m.Diagnostics) == 0 || m.Diagnostics[0].Code != "INVALID_TICKET" {
			t.Fatalf("%s: %+v", value, m)
		}
		title := "Must not publish"
		m = UpdateWithOptions(root, id, UpdateOptions{Changes: ticket.Changes{Title: &title, NoParent: true, Fields: opts.Fields}})
		if m.Committed || len(m.Diagnostics) == 0 || m.Diagnostics[0].Code != "INVALID_TICKET" {
			t.Fatalf("%s: %+v", value, m)
		}
		assertProjectUnchanged(t, root, before)
	}
	opts := UpdateOptions{Changes: ticket.Changes{Fields: customFields(t, "estimate=1.23456789012345678901234567890", "opaque=null"), RemoveFields: []string{"review", "absent"}, NoParent: true}}
	m = UpdateWithOptions(root, id, opts)
	if len(m.Diagnostics) > 0 || !m.Changed || m.Ticket.Parent != nil {
		t.Fatalf("%+v", m)
	}
	af := schema.Map(schema.Map(m.Ticket.Node)["fields"])
	if af["review"] != nil || af["opaque"].Tag != "!!null" || !bytes.Equal(before.ByID[id].Body, m.Ticket.Body) {
		t.Fatal("incorrect field update")
	}
	info, _ := os.Stat(filepath.Join(root, m.Ticket.Path))
	if info.Mode().Perm() != before.Files[m.Ticket.Path].Info.Mode().Perm() {
		t.Fatal("permissions changed")
	}
	before = project.Load(root)
	m = UpdateWithOptions(root, id, opts)
	if len(m.Diagnostics) > 0 || m.Changed {
		t.Fatal("no-op rewrote", m.Diagnostics)
	}
	assertProjectUnchanged(t, root, before)
	// Updating configured definitions can invalidate data. Mutations still refuse
	// the whole project, even an attempted removal of the offending field.
	if err := os.WriteFile(filepath.Join(root, project.ConfigPath), []byte(strings.Replace(config, "type: number", "type: boolean", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	before = project.Load(root)
	m = UpdateWithOptions(root, id, UpdateOptions{Changes: ticket.Changes{RemoveFields: []string{"estimate"}, RemoveDependencies: []string{testID}}})
	if len(m.Diagnostics) == 0 || m.Committed {
		t.Fatal("mutated invalid project")
	}
	assertProjectUnchanged(t, root, before)
}

func TestRelationshipAndFieldLockAndNoops(t *testing.T) {
	root := testProject(t)
	before := project.Load(root)
	for _, changes := range []ticket.Changes{
		{NoParent: true}, {RemoveDependencies: []string{"wrk-deadbeef"}},
		{RemoveFields: []string{"missing", "missing"}}, {Fields: customFields(t, "huge=!!int 123456789012345678901234567890")},
	} {
		m := UpdateWithOptions(root, testID, UpdateOptions{Changes: changes})
		if len(m.Diagnostics) > 0 || m.Changed {
			t.Fatalf("%+v", m)
		}
		assertProjectUnchanged(t, root, before)
	}
	lock, err := Acquire(filepath.Join(root, ".wrk"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	fields := customFields(t, "x=new")
	for _, m := range []Mutation{
		Create(root, CreateOptions{Title: "Locked", Dependencies: []string{testID}, Fields: fields}),
		UpdateWithOptions(root, testID, UpdateOptions{Changes: ticket.Changes{NoParent: true, Fields: fields}}),
	} {
		if len(m.Diagnostics) == 0 || m.Diagnostics[0].Code != "BUSY" || m.Committed {
			t.Fatalf("%+v", m)
		}
	}
	assertProjectUnchanged(t, root, before)
}
