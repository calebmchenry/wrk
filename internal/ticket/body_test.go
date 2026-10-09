package ticket

import (
	"bytes"
	"errors"
	"testing"
	"wrk/internal/schema"
)

func TestBodyReplacementAndCombinedPreservation(t *testing.T) {
	source := []byte("---\r\nid: wrk-12345678\r\ntitle: &title Old\r\nstatus: todo\r\nfields: {old: *title, huge: !!int 123456789012345678901234567890, loop: &loop [*loop]}\r\n---\r\nOriginal body")
	before, ds := Parse(source, "fixture")
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	for _, body := range []string{"", "\n", "no final newline", "\r\nUnicode 🦊\r\n---\nbody"} {
		for _, combined := range []bool{false, true} {
			title := "New"
			changes := Changes{Body: &body}
			if combined {
				changes.Title = &title
			}
			data, changed, err := PatchChanges(before, changes)
			if err != nil || !changed {
				t.Fatalf("body %q combined %v: %v, %v", body, combined, changed, err)
			}
			after, ds := Parse(data, "fixture")
			if len(ds) > 0 || string(after.Body) != body {
				t.Fatalf("body %q: got %q, %v", body, after.Body, ds)
			}
			for key, value := range schema.Map(before.Node) {
				if key != "title" || !combined {
					if !Equal(value, schema.Map(after.Node)[key]) {
						t.Fatalf("unrelated %s changed", key)
					}
				}
			}
			noop, changed, err := PatchChanges(after, changes)
			if err != nil || changed || !bytes.Equal(noop, data) {
				t.Fatalf("no-op changed bytes: %v", err)
			}
		}
	}
	invalid := "\xff"
	if _, changed, err := PatchChanges(before, Changes{Body: &invalid}); !errors.Is(err, ErrInvalidBody) || changed {
		t.Fatalf("invalid input: changed=%v err=%v", changed, err)
	}
}

func TestBodyAfterDelimiterAtEOF(t *testing.T) {
	before, ds := Parse([]byte("---\nid: wrk-12345678\ntitle: T\nstatus: todo\n---"), "fixture")
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	body := "first body"
	data, changed, err := PatchChanges(before, Changes{Body: &body})
	if err != nil || !changed {
		t.Fatal(changed, err)
	}
	after, ds := Parse(data, "fixture")
	if len(ds) > 0 || string(after.Body) != body {
		t.Fatal(string(data), ds)
	}
}
