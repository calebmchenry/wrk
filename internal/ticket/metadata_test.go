package ticket

import (
	"bytes"
	"slices"
	"testing"
	"wrk/internal/schema"
)

func TestPriorityAndReplacementPreserveAliases(t *testing.T) {
	for _, front := range []string{
		"priority: &priority high\nlabels: &labels [&label old, keep, keep]\nfields: {priority: *priority, labels: *labels, label: *label, precise: !!int 123456789012345678901234567890, opaque: !tag [one, two], loop: &loop [*loop]}\n",
		"fields: {priority: &priority high, labels: &labels [old, keep, keep]}\npriority: *priority\nlabels: *labels\n",
		"labels: &labels [&priority high, keep, keep]\npriority: *priority\nfields: {priority: *priority, labels: *labels}\n",
		"priority: &priority high\nlabels: [*priority, keep, keep]\nfields: {priority: *priority}\n",
		"",
	} {
		for _, wantLabels := range [][]string{{"z", "a", "a"}, {}} {
			t.Run(front+"/"+string(rune(len(wantLabels)+'0')), func(t *testing.T) {
				src := []byte("---\r\nid: wrk-12345678\r\ntitle: Before\r\nstatus: todo\r\n" + front + "---\r\nExact 🦊\r\n---\nno newline")
				before, ds := Parse(src, "fixture")
				if len(ds) > 0 {
					t.Fatal(ds)
				}
				title, status, priority := "After", "blocked", "urgent"
				opts := Changes{Title: &title, Status: &status, Priority: &priority, LabelsSet: true, Labels: wantLabels}
				data, changed, err := PatchChanges(before, opts)
				if err != nil || !changed {
					t.Fatalf("changed %t: %v", changed, err)
				}
				after, ds := Parse(data, "fixture")
				ds = append(ds, Validate(after, nil)...)
				if len(ds) > 0 || !bytes.Equal(before.Body, after.Body) || after.Priority != priority || after.Title != title || after.Status != status || !slices.Equal(after.Labels, wantLabels) {
					t.Fatalf("%+v %v", after, ds)
				}
				bm, am := schema.Map(before.Node), schema.Map(after.Node)
				for key, node := range bm {
					if key != "title" && key != "status" && key != "priority" && key != "labels" && !Equal(node, am[key]) {
						t.Fatal("changed unrelated field", key)
					}
				}
				if am["parent"] != nil || am["depends_on"] != nil {
					t.Fatal("inserted unrelated defaults")
				}
				if front == "" && len(wantLabels) == 0 && am["labels"] != nil {
					t.Fatal("clear inserted absent labels")
				}
				noop, changed, err := PatchChanges(after, opts)
				if err != nil || changed || !bytes.Equal(noop, data) {
					t.Fatal("exact replacement rewrote", err)
				}
			})
		}
	}
}

func TestMetadataEffectiveNoopsAndOmissions(t *testing.T) {
	normal := "normal"
	for _, front := range []string{"", "priority: normal\nlabels: []\n", "priority: normal\nlabels: [z, a, a]\n"} {
		src := []byte("---\nid: wrk-12345678\ntitle: T\nstatus: todo\n" + front + "---\n")
		before, _ := Parse(src, "fixture")
		labels, _ := schema.StringList(schema.Map(before.Node)["labels"], true)
		opts := Changes{Priority: &normal, LabelsSet: true, Labels: labels}
		got, changed, err := PatchChanges(before, opts)
		if err != nil || changed || !bytes.Equal(src, got) {
			t.Fatal("effective no-op rewrote", err)
		}
		title := "New"
		opts.Title = &title
		got, changed, err = PatchChanges(before, opts)
		if err != nil || !changed {
			t.Fatal(err)
		}
		after, _ := Parse(got, "fixture")
		if front == "" && (schema.Map(after.Node)["priority"] != nil || schema.Map(after.Node)["labels"] != nil) {
			t.Fatal("combined update inserted no-op fields")
		}
	}
	// A replacement's source order is intentional even though summaries sort it.
	src := []byte("---\nid: wrk-12345678\ntitle: T\nstatus: todo\nlabels: [z, a, a]\n---\nbody")
	before, _ := Parse(src, "fixture")
	got, changed, err := PatchChanges(before, Changes{LabelsSet: true, Labels: []string{"a", "a", "z"}})
	if err != nil || !changed {
		t.Fatal("reordered replacement ignored", err)
	}
	after, _ := Parse(got, "fixture")
	labels, _ := schema.StringList(schema.Map(after.Node)["labels"], true)
	if !slices.Equal(labels, []string{"a", "a", "z"}) || schema.Map(after.Node)["priority"] != nil {
		t.Fatal(labels)
	}
}
