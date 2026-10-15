package ticket

import (
	"strings"
	"testing"
	"time"

	"wrk/internal/schema"
)

func TestCreatedAtValidation(t *testing.T) {
	for _, value := range []string{
		"2026-10-15T18:42:03Z", `"2026-10-15T18:42:03Z"`,
		"2024-02-29T23:59:59.123456789-04:30", "2026-10-15T18:42:03+05:30",
		"!!timestamp 2026-10-15T18:42:03Z", "&date 2026-10-15T18:42:03Z",
	} {
		t.Run(value, func(t *testing.T) {
			item, ds := Parse([]byte("---\nid: wrk-12345678\ntitle: T\nstatus: todo\ncreated_at: "+value+"\n---\n"), "fixture.md")
			ds = append(ds, Validate(item, nil)...)
			if len(ds) > 0 || item.CreatedAt == nil || *item.CreatedAt != schema.Resolve(schema.Map(item.Node)["created_at"]).Value {
				t.Fatalf("%+v: %v", item, ds)
			}
		})
	}
	for _, value := range []string{
		"null", "''", "123", "true", "[]", "{}", "!custom 2026-10-15T18:42:03Z",
		"yesterday", "2026-10-15", "2026-10-15 18:42:03Z", "2026-10-15T18:42:03",
		"2026-10-15t18:42:03z", "2026-10-15T8:42:03Z", "2026-10-15T18:42:03,5Z",
		"2026-02-29T18:42:03Z", "2026-13-15T18:42:03Z", "2026-10-15T24:00:00Z",
		"2026-10-15T18:42:60Z", "2026-10-15T18:42:03+24:00", "2026-10-15T18:42:03+00:60",
		"2026-10-15T18:42:03.1234567890Z", "2026-10-15T18:42:03Z trailing",
	} {
		t.Run(value, func(t *testing.T) {
			item, ds := Parse([]byte("---\nid: wrk-12345678\ntitle: T\nstatus: todo\ncreated_at: "+value+"\n---\n"), "fixture.md")
			ds = append(ds, Validate(item, nil)...)
			if len(ds) != 1 || ds[0].Code != "INVALID_TICKET" || ds[0].Field != "created_at" || ds[0].Path != "fixture.md" || ds[0].Line != 5 || !strings.Contains(ds[0].Message, "omit the field if unknown") || item.CreatedAt != nil {
				t.Fatalf("%+v: %v", item, ds)
			}
		})
	}
}

func TestCreatedAtSerialization(t *testing.T) {
	when := time.Date(2026, 10, 15, 14, 42, 3, 123456789, time.FixedZone("local", -4*60*60))
	data, err := CreateWithRelated("wrk-12345678", "T", "normal", nil, nil, nil, nil, nil, []byte("body"), when)
	if err != nil {
		t.Fatal(err)
	}
	item, ds := Parse(data, "fixture.md")
	ds = append(ds, Validate(item, nil)...)
	if len(ds) > 0 || item.CreatedAt == nil || *item.CreatedAt != "2026-10-15T18:42:03Z" || !strings.Contains(string(data), "created_at: 2026-10-15T18:42:03Z\n") {
		t.Fatalf("%s: %v", data, ds)
	}
}

func TestCreatedAtPreservedByEdits(t *testing.T) {
	for _, front := range []string{
		"", "created_at: 2026-10-15T18:42:03Z\n", "created_at: '2026-10-15T14:42:03.123456789-04:00'\n",
		"created_at: &date 2026-10-15T18:42:03Z\nfields: {date: *date}\n",
		"fields: {date: &date 2026-10-15T18:42:03Z}\ncreated_at: *date\n",
	} {
		t.Run(front, func(t *testing.T) {
			item, ds := Parse([]byte("---\nid: wrk-12345678\ntitle: T\nstatus: todo\n"+front+"---\nbody"), "fixture.md")
			if ds = append(ds, Validate(item, nil)...); len(ds) > 0 {
				t.Fatal(ds)
			}
			title, status, body, priority, parent := "New", "done", "replacement", "high", "wrk-00000001"
			for _, changes := range []Changes{
				{Body: &body}, {Title: &title, Status: &status, Priority: &priority, LabelsSet: true, Labels: []string{"tag"}},
				{Parent: &parent, AddDependencies: []string{parent}, AddRelated: []string{parent}}, {NoParent: true, RemoveDependencies: []string{parent}, NoRelated: true},
			} {
				data, _, err := PatchChanges(item, changes)
				if err != nil {
					encoded, _ := encode(item.Node, item.Body)
					t.Fatalf("%v: %s", err, encoded)
				}
				after, ds := Parse(data, "fixture.md")
				ds = append(ds, Validate(after, nil)...)
				if len(ds) > 0 || !Equal(schema.Map(item.Node)["created_at"], schema.Map(after.Node)["created_at"]) || (item.CreatedAt == nil) != (after.CreatedAt == nil) {
					t.Fatalf("timestamp changed: %s; %v", data, ds)
				}
				item = after
			}
		})
	}
}
