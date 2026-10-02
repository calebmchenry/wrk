package ticket

import (
	"bytes"
	"strings"
	"testing"
	"wrk/internal/schema"
)

func TestPatchPreservesAliasesAndOpaqueValues(t *testing.T) {
	src := []byte("---\r\nid: wrk-12345678\r\ntitle: &title Old\r\nstatus: &status todo\r\nfields:\r\n  oldTitle: *title\r\n  oldStatus: *status\r\n  huge: !!int 1234567890123456789012345678901234567890\r\n  nested: [!opaque hello, {answer: 42, yes: yes}]\r\n  recursive: &loop [*loop]\r\n---\r\n\nUnicode 🦊\r\n---\nno newline")
	before, ds := Parse(src, "ticket")
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	title, status := "New", "done"
	data, changed, err := Patch(before, &title, &status)
	if err != nil || !changed {
		t.Fatalf("%v %v", changed, err)
	}
	after, ds := Parse(data, "ticket")
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	if !bytes.Equal(before.Body, after.Body) {
		t.Fatal("body changed")
	}
	if !Equal(schema.Map(before.Node)["fields"], schema.Map(after.Node)["fields"]) {
		t.Fatal("fields changed")
	}
}
func TestNoopAndDelimiters(t *testing.T) {
	for _, body := range []string{"", "\n", "---\nlater", "no terminal newline"} {
		src := []byte("---\nid: wrk-12345678\ntitle: T\nstatus: todo\n---\n" + body)
		ticket, ds := Parse(src, "t")
		if len(ds) > 0 || string(ticket.Body) != body {
			t.Fatalf("%v %q", ds, ticket.Body)
		}
		title := "T"
		got, changed, err := Patch(ticket, &title, nil)
		if err != nil || changed || !bytes.Equal(got, src) {
			t.Fatal("no-op rewrote")
		}
	}
	eof, ds := Parse([]byte("---\ntitle: T\n---"), "t")
	if len(ds) > 0 || len(eof.Body) != 0 {
		t.Fatal(ds)
	}
}

func TestAliasMutationCombinations(t *testing.T) {
	for _, front := range []string{
		"title: &t todo\nstatus: *t\nfields: {old: *t}\n",
		"title: *value\nstatus: todo\n",
		"title: &value Old\nstatus: todo\nfields: { ? *value : !opaque data }\n",
	} {
		prefix := "id: wrk-12345678\n"
		if strings.HasPrefix(front, "title: *value") {
			prefix += "fields: {value: &value Old}\n"
		}
		src := []byte("---\n" + prefix + front + "---\nbody")
		tkt, ds := Parse(src, "fixture")
		if len(ds) > 0 {
			t.Fatal(ds)
		}
		title := "Changed"
		out, _, err := Patch(tkt, &title, nil)
		if err != nil {
			t.Fatal(err)
		}
		after, ds := Parse(out, "fixture")
		if len(ds) > 0 {
			t.Fatal(ds)
		}
		for key, before := range schema.Map(tkt.Node) {
			if key != "title" && !Equal(before, schema.Map(after.Node)[key]) {
				t.Fatalf("%s changed", key)
			}
		}
	}
}
func TestMalformedTicketBytes(t *testing.T) {
	for _, src := range [][]byte{[]byte("title: no delimiters"), []byte("---\nid: x"), []byte("---\ntitle: x\n---\n\xff"), []byte("---\ntitle: [\n---\n"), []byte("---\n[]\n---\n"), []byte("\xef\xbb\xbf---\nid: x\n---\n")} {
		if _, ds := Parse(src, "fixture"); len(ds) == 0 {
			t.Fatalf("accepted %q", src)
		}
	}
}
