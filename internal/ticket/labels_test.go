package ticket

import (
	"bytes"
	"slices"
	"testing"
	"wrk/internal/schema"
)

func TestLabelPatchesPreserveData(t *testing.T) {
	for _, front := range []string{
		"labels: &labels [&item old, keep, keep]\nfields: {oldLabels: *labels, oldItem: *item, opaque: !tag [1, 2], loop: &loop [*loop]}\n",
		"fields: {labels: &labels [old, keep, keep]}\nlabels: *labels\n",
		"labels: [old, keep, keep]\n",
		"",
	} {
		t.Run(front, func(t *testing.T) {
			src := []byte("---\r\nid: wrk-12345678\r\ntitle: &title Old\r\nstatus: todo\r\n" + front + "---\r\nExact 🦊\r\n---\nno newline")
			before, ds := Parse(src, "fixture")
			if len(ds) > 0 {
				t.Fatal(ds)
			}
			title, status := "New", "blocked"
			data, changed, err := PatchChanges(before, Changes{Title: &title, Status: &status, AddLabels: []string{"burn", "burn"}, RemoveLabels: []string{"old", "missing", "old"}})
			if err != nil || !changed {
				t.Fatalf("%v %v", changed, err)
			}
			after, ds := Parse(data, "fixture")
			ds = append(ds, Validate(after, nil)...)
			if len(ds) > 0 {
				t.Fatal(ds)
			}
			want := []string{"keep", "keep", "burn"}
			if front == "" {
				want = []string{"burn"}
			}
			if !slices.Equal(after.Labels, want) || after.Title != title || after.Status != status || !bytes.Equal(before.Body, after.Body) {
				t.Fatalf("%+v", after)
			}
			bm, am := schema.Map(before.Node), schema.Map(after.Node)
			for key, node := range bm {
				if key != "labels" && key != "title" && key != "status" && !Equal(node, am[key]) {
					t.Fatalf("changed %s", key)
				}
			}
			if am["priority"] != nil || am["depends_on"] != nil || am["parent"] != nil {
				t.Fatal("inserted defaults")
			}
			noop, changed, err := PatchChanges(after, Changes{AddLabels: []string{"burn", "keep"}, RemoveLabels: []string{"absent"}})
			if front != "" && (err != nil || changed || !bytes.Equal(noop, data)) {
				t.Fatalf("no-op rewrote: %v", err)
			}
		})
	}
}

func TestLabelNoopsAndRemoval(t *testing.T) {
	for _, front := range []string{"", "labels: []\n", "labels: [keep, keep]\n"} {
		src := []byte("---\nid: wrk-12345678\ntitle: T\nstatus: todo\n" + front + "---\nbody")
		before, _ := Parse(src, "fixture")
		got, changed, err := PatchChanges(before, Changes{RemoveLabels: []string{"absent", "absent"}})
		if err != nil || changed || !bytes.Equal(got, src) {
			t.Fatal("remove absent rewrote", err)
		}
		if front != "labels: [keep, keep]\n" {
			continue
		}
		got, changed, err = PatchChanges(before, Changes{RemoveLabels: []string{"keep"}})
		after, ds := Parse(got, "fixture")
		labels, ok := schema.StringList(schema.Map(after.Node)["labels"], true)
		if err != nil || !changed || len(ds) > 0 || !ok || len(labels) != 0 {
			t.Fatal("did not remove all duplicates", err, ds)
		}
	}
}
