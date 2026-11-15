package mapimport

import (
	"strings"
	"testing"
)

func TestParse_Errors(t *testing.T) {
	ctl := func(body string) string {
		return "entities:\n  controls:\n    type: control\n    select: $.c\n" + body
	}
	keyed := "    key: oscal_id\n    properties:\n      oscal_id: id\n"
	cases := []struct {
		name, src, want string
	}{
		{"unknown top field", "nope: 1\nentities: {}\n", "field nope not found"},
		{"no entities", "entities: {}\n", "declares no mapping"},
		{"bad format", "format: toml\n" + ctl(keyed), `format "toml"`},
		{"max_bytes too high", "max_bytes: 999999999\n" + ctl(keyed), "max_bytes"},
		{"max_records too high", "max_records: 999999999\n" + ctl(keyed), "max_records"},
		{"bad mapping name", "entities:\n  Bad:\n    type: control\n    select: $\n" + keyed, "invalid name"},
		{"unknown type", "entities:\n  x:\n    type: nope\n    select: $\n", `type "nope" is not an entity type`},
		{"bad select", ctl(keyed)[:0] + "entities:\n  c:\n    type: control\n    select: '$x'\n" + keyed, "select:"},
		{"no identity", ctl(""), "needs a key property or an id template"},
		{"key not unique", ctl("    key: title\n    properties:\n      title: t\n"), "must be unique"},
		{"key not mapped", ctl("    key: oscal_id\n"), "not mapped under properties"},
		{"id template on generated ids", ctl("    id: 'x-{{id}}'\n"), "generates its ids"},
		{"manual ids need an id", "entities:\n  d:\n    type: doc\n    select: $\n    key: title\n", "needs an id template"},
		{"unknown property", ctl(keyed + "      nope: x\n"), "has no such property"},
		{"property in scope", ctl(keyed + "      title: t\n    scope: {title: x}\n"), "is also in scope"},
		{"scope list", ctl(keyed + "    scope: {tags: x}\n"), "plain single-valued"},
		{"empty scope value", ctl(keyed + "    scope: {catalog: ''}\n"), "needs a value"},
		{"scope of the wrong type", ctl(keyed + "    scope: {weight: heavy}\n"), "is not an integer"},
		{"overlapping scopes", "entities:\n  a:\n    type: control\n    select: $.a\n" + keyed +
			"  b:\n    type: control\n    select: $.b\n" + keyed, "give them scopes that differ"},
		{"prune next to a sibling", ctl(keyed + "    relations:\n" +
			"      - {type: refines, target: controls, key: x, prune: true}\n" +
			"      - {name: other, type: refines, target: controls, key: y}\n"), "prune cannot be set"},
		{"two value kinds", ctl(keyed + "      title: {path: a, value: b}\n"), "exactly one of"},
		{"bad const", ctl(keyed + "      title: {value: [a]}\n"), "string, number or boolean"},
		{"unknown value field", ctl(keyed + "      title: {path: a, transform: x}\n"), "field transform not found"},
		{"bad content", ctl(keyed + "    content: {template: plain}\n"), "content:"},
		{"unknown relation", ctl(keyed + "    relations:\n      - {type: nope, target: controls, key: x}\n"),
			"not a relation type"},
		{"no target", ctl(keyed + "    relations:\n      - {type: refines, key: x}\n"), "needs target or target_type"},
		{"both targets", ctl(keyed + "    relations:\n      - {type: refines, target: controls, target_type: control, " +
			"target_key: oscal_id, key: x}\n"), "not both"},
		{"unknown target", ctl(keyed + "    relations:\n      - {type: refines, target: nope, key: x}\n"),
			"not an entity mapping"},
		{"target_type without key", ctl(keyed + "    relations:\n      - {type: refines, target_type: control, key: x}\n"),
			"needs target_key"},
		{"target_key not unique", ctl(keyed + "    relations:\n      - {type: mitigates, target_type: risk, " +
			"target_key: title, key: x}\n"), "must be a unique"},
		{"wrong endpoints", ctl(keyed + "    relations:\n      - {type: part-of, target: controls, key: x}\n"),
			"does not connect control to control"},
		{"bad direction", ctl(keyed + "    relations:\n      - {type: refines, target: controls, key: x, direction: up}\n"),
			"direction"},
		{"relation without key", ctl(keyed + "    relations:\n      - {type: refines, target: controls}\n"), "needs key"},
		{"duplicate relation name", ctl(keyed + "    relations:\n      - {type: refines, target: controls, key: a}\n" +
			"      - {type: refines, target: controls, key: b}\n"), "used twice"},
		{"hook outside project", ctl(keyed + "    hook: ../x.lua\n"), "project-relative"},
		{"hook not lua", ctl(keyed + "    hook: scripts/x.py\n"), ".lua file"},
		{"hook bad function", ctl(keyed + "    hook: {script: x.lua, function: 'a-b'}\n"), "not a Lua name"},
	}
	meta := testMeta(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse("test", []byte(tc.src), meta)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Parse = %v, want %q", err, tc.want)
			}
			if !strings.HasPrefix(err.Error(), "imports/test.yaml: ") && !strings.Contains(tc.want, "invalid") {
				t.Fatalf("error does not name the file: %v", err)
			}
		})
	}
}

func TestParse_Valid(t *testing.T) {
	meta := testMeta(t)
	m, err := Parse("oscal", []byte(readTestdata(t, "oscal/mapping.yaml")), meta)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(m.Names(), ","); got != "controls,groups" {
		t.Fatalf("names = %s", got)
	}
	ctl := m.Entities["controls"]
	if ctl.Name() != "controls" || ctl.Hook.function() != DefaultHookFunction || ctl.Relations[0].Name != "part-of" {
		t.Fatalf("defaults not applied: %+v", ctl)
	}
	if _, err := Parse("Bad Name", nil, meta); err == nil {
		t.Fatal("an invalid name was accepted")
	}
	if _, err := Parse("x", nil, nil); err == nil {
		t.Fatal("a nil schema was accepted")
	}
	if FilePath("x") != "imports/x.yaml" {
		t.Fatal(FilePath("x"))
	}
}
