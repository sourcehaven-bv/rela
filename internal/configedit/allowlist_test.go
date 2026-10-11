package configedit

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestAllowlist_ClassifiesEveryKey fails when a configuration struct gains a
// key nobody has decided about. Add it to editable or to locked.
func TestAllowlist_ClassifiesEveryKey(t *testing.T) {
	for file, typ := range map[File]reflect.Type{
		SchemaFile:    reflect.TypeFor[metamodel.Metamodel](),
		DataEntryFile: reflect.TypeFor[dataentryconfig.Config](),
	} {
		for _, p := range yamlPaths(typ) {
			if strings.Contains(p, " (recursive") {
				continue // a nested navigation entry, classified as "nav"
			}
			path := normalize(file, strings.Split(p, "."))
			ed := anyMatch(editable[file], path, match)
			lo := anyMatch(locked[file], path, match)
			switch {
			case !ed && !lo:
				t.Errorf("%s: %s is neither editable nor locked", file, p)
			case ed && lo:
				t.Errorf("%s: %s is both editable and locked", file, p)
			}
		}
	}
}

func treeOf(t *testing.T, yamlText string) any {
	t.Helper()
	return wire(t, readTree(t, []byte(yamlText)))
}

func TestCheckEdit(t *testing.T) {
	const schema = `entities:
  ticket:
    label: Ticket
    properties:
      title: {type: string}
      file:
        type: file
        scan_cmd: [clamscan, "{in}"]
validations:
  - name: v1
    entity_type: ticket
automations:
  - name: a1
    on: {entity: ticket, property: status, becomes: done}
    do:
      - set: done_at
        value: "{{today}}"
`
	tests := []struct {
		name string
		edit string
		want []string
	}{
		{"unchanged", schema, nil},
		{"add a property", strings.Replace(schema, "      title: {type: string}\n", "      title: {type: string}\n      due: {type: date, required: true}\n", 1), nil},
		{"rename a label", strings.Replace(schema, "label: Ticket", "label: Issue", 1), nil},
		{"remove a property holding a scan", strings.Replace(schema, "      file:\n        type: file\n        scan_cmd: [clamscan, \"{in}\"]\n", "", 1), []string{"entities.ticket.properties.file removed"}},
		{"change scan_cmd", strings.Replace(schema, "clamscan", "sh", 1), []string{"entities.ticket.properties.file.scan_cmd changed"}},
		{"remove scan_cmd alone", strings.Replace(schema, "        scan_cmd: [clamscan, \"{in}\"]\n", "", 1), []string{"entities.ticket.properties.file.scan_cmd removed"}},
		{"add scan_cmd", strings.Replace(schema, "      title: {type: string}\n", "      title: {type: string, scan_cmd: [sh, -c, x]}\n", 1), []string{"entities.ticket.properties.title.scan_cmd added"}},
		{"add a transform command", strings.Replace(schema, "      title: {type: string}\n", "      title:\n        type: string\n        transform: [{cmd: [sh]}]\n", 1), []string{"entities.ticket.properties.title.transform.0.cmd added"}},
		{"add lua to a validation", strings.Replace(schema, "    entity_type: ticket\n", "    entity_type: ticket\n    lua: os.exit()\n", 1), []string{"validations.0.lua added"}},
		{"add lua_file to a validation", strings.Replace(schema, "    entity_type: ticket\n", "    entity_type: ticket\n    lua_file: x.lua\n", 1), []string{"validations.0.lua_file added"}},
		{"new validation with lua", strings.Replace(schema, "automations:\n", "  - name: v2\n    lua: x\nautomations:\n", 1), []string{"validations.1.lua added"}},
		{"new automation without code", schema + "  - name: a2\n    on: {created: true}\n    do: [{set: x, value: y}]\n", nil},
		{"add lua to an automation action", strings.Replace(schema, "        value: \"{{today}}\"\n", "        value: \"{{today}}\"\n      - lua: x\n", 1), []string{"automations.0.do.1.lua added"}},
		{"add acl bypass", strings.Replace(schema, "      - set: done_at\n", "      - allow_acl_bypass: read+write\n        set: done_at\n", 1), []string{"automations.0.do.0.allow_acl_bypass added"}},
		{"add capabilities", strings.Replace(schema, "      - set: done_at\n", "      - capabilities: {http: true}\n        set: done_at\n", 1), []string{"automations.0.do.0.capabilities.http added"}},
		{"add an attachments block", schema + "attachments:\n  scan_cmd: [sh]\n", []string{"attachments.scan_cmd added"}},
		{"add a query scope", strings.Replace(schema, "    label: Ticket\n", "    label: Ticket\n    query_scopes:\n      open: \"entity.status ~= 'done'\"\n", 1), nil},
		{"turn comments on", schema + "comments: {enabled: true, on: [ticket]}\n", nil},
		{"add an include", schema + "includes: [other.yaml]\n", []string{"includes added"}},
		{"add a transform", schema + "transforms:\n  pdf: {command: [sh]}\n", []string{"transforms.pdf.command added"}},
		{"turn an editable value into a mapping", schema + "types:\n  st:\n    default: {scan_cmd: [sh]}\n",
			[]string{"types.st.default.scan_cmd added"}},
		{"add a transition guard", schema + "types:\n  st:\n    values: [a, b]\n    transitions: [{from: a, to: b, guard: admin}]\n", []string{"types.st.transitions.0.guard added"}},
	}
	before := treeOf(t, schema)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, v := range CheckEdit(SchemaFile, before, treeOf(t, tc.edit)) {
				got = append(got, v.Path+" "+v.Change)
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCheckEdit_Protective pins that an item holding a restriction cannot
// be removed, so it cannot be removed and added back without it. An item
// holding only code may still be removed.
func TestCheckEdit_Protective(t *testing.T) {
	const schema = `entities:
  ticket:
    label: Ticket
    properties:
      title:
        type: string
        transform: [{cmd: [pandoc]}]
      file:
        type: file
        accept: [application/pdf]
types:
  st:
    values: [a, b]
    transitions:
      - {from: a, to: b, guard: approve}
      - {from: b, to: a}
`
	const de = `app:
  name: Tickets
  max_attachment_bytes: 1000
navigation:
  - label: Admin
    list: all
    permission: admin:read
  - label: All
    list: all
`
	tests := []struct {
		name       string
		file       File
		base, edit string
		want       []string
	}{
		{"remove a property holding code", SchemaFile, schema,
			strings.Replace(schema, "      title:\n        type: string\n        transform: [{cmd: [pandoc]}]\n", "", 1), nil},
		{"remove a property holding accept", SchemaFile, schema,
			strings.Replace(schema, "      file:\n        type: file\n        accept: [application/pdf]\n", "", 1),
			[]string{"entities.ticket.properties.file removed"}},
		{"remove the entity type", SchemaFile, schema,
			strings.Replace(schema, "  ticket:\n    label: Ticket\n", "  other:\n    label: Other\n", 1),
			[]string{
				"entities.other.properties.file.accept added",
				"entities.other.properties.title.transform.0.cmd added",
				"entities.ticket removed",
			}},
		{"remove an unguarded transition", SchemaFile, schema,
			strings.Replace(schema, "      - {from: b, to: a}\n", "", 1), nil},
		{"remove a guarded transition", SchemaFile, schema,
			strings.Replace(schema, "      - {from: a, to: b, guard: approve}\n", "", 1),
			[]string{
				"types.st.transitions.0.from changed",
				"types.st.transitions.0.guard removed",
				"types.st.transitions.0.to changed",
			}},
		{"move a guarded transition", SchemaFile, schema,
			strings.Replace(schema, "{from: a, to: b, guard: approve}", "{from: b, to: b, guard: approve}", 1),
			[]string{"types.st.transitions.0.from changed"}},
		{"move an unguarded transition", SchemaFile, schema,
			strings.Replace(schema, "{from: b, to: a}", "{from: a, to: a}", 1), nil},
		{"relabel a guarded transition", SchemaFile, schema,
			strings.Replace(schema, "guard: approve}", "guard: approve, label: Approve}", 1), nil},
		{"retarget a validation holding lua", SchemaFile, schema + "validations:\n  - {name: v, entity_type: ticket, lua_file: v.lua}\n",
			schema + "validations:\n  - {name: v, entity_type: other, lua_file: v.lua}\n",
			[]string{"validations.0.entity_type changed"}},
		{"retarget an automation holding lua", SchemaFile,
			schema + "automations:\n  - {name: a, on: {entity: ticket}, do: [{lua: x}]}\n",
			schema + "automations:\n  - {name: a, on: {entity: person}, do: [{lua: x}]}\n",
			[]string{"automations.0.on changed"}},
		{"retarget an automation without code", SchemaFile,
			schema + "automations:\n  - {name: a, on: {entity: ticket}, do: [{set: x, value: y}]}\n",
			schema + "automations:\n  - {name: a, on: {entity: person}, do: [{set: x, value: y}]}\n", nil},
		{"remove a nav entry holding a permission", DataEntryFile, de,
			strings.Replace(de, "  - label: Admin\n    list: all\n    permission: admin:read\n", "", 1),
			[]string{"navigation.0.permission removed"}},
		{"remove the app block holding a limit", DataEntryFile, de,
			strings.Replace(de, "app:\n  name: Tickets\n  max_attachment_bytes: 1000\n", "", 1),
			[]string{"app removed"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, v := range CheckEdit(tc.file, treeOf(t, tc.base), treeOf(t, tc.edit)) {
				got = append(got, v.Path+" "+v.Change)
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCheckEdit_DataEntry(t *testing.T) {
	const de = `app:
  name: Tickets
lists:
  all:
    entity_type: ticket
    export_render: render.lua
    columns:
      - property: title
navigation:
  - label: Work
    items:
      - label: All
        list: all
        permission: tickets:read
`
	tests := []struct {
		name string
		edit string
		want []string
	}{
		{"rename a nested nav entry", strings.Replace(de, "label: All", "label: Everything", 1), nil},
		{"remove a nested nav permission", strings.Replace(de, "        permission: tickets:read\n", "", 1), []string{"navigation.0.items.0.permission removed"}},
		{"add a nav action", strings.Replace(de, "  - label: Work\n", "  - label: Work\n    action: deploy\n", 1), []string{"navigation.0.action added"}},
		{"change export_render", strings.Replace(de, "render.lua", "evil.lua", 1), []string{"lists.all.export_render changed"}},
		{"add a column", strings.Replace(de, "      - property: title\n", "      - property: title\n      - property: status\n        sortable: true\n", 1), nil},
		{"add a webhook", de + "webhooks:\n  w: {script: x.lua}\n", []string{"webhooks.w.script added"}},
		{"add a command", de + "commands:\n  c: {command: [sh]}\n", []string{"commands.c.command added"}},
		{"add a detail view", de + "views:\n  t:\n    title: Ticket\n    entry: {type: ticket}\n    traverse: [{from: entry, follow: blocks, collect_as: b}]\n    sections: [{source: b, display: table}]\n", nil},
		{"add a view with export_render", de + "views:\n  t: {title: T, export_render: x.lua}\n", []string{"views.t.export_render added"}},
		{"add a calendar", de + "calendars:\n  due:\n    sources: [{entity_type: ticket, date: due}]\n", nil},
		{"add a timeline", de + "gantts:\n  plan: {hierarchy: [contains]}\n", nil},
		{"set the app logo injection", strings.Replace(de, "  name: Tickets\n", "  name: Tickets\n  disable_custom_injection: false\n", 1), []string{"app.disable_custom_injection added"}},
	}
	before := treeOf(t, de)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, v := range CheckEdit(DataEntryFile, before, treeOf(t, tc.edit)) {
				got = append(got, v.Path+" "+v.Change)
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The real files must pass unchanged, and a list item moved around keeps
// passing as long as only its editable keys differ.
func TestCheckEdit_RealFilesUnchanged(t *testing.T) {
	for file, name := range map[File]string{SchemaFile: "testdata/schema.yaml", DataEntryFile: "testdata/data-entry.yaml"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		tree := readTree(t, src)
		if v := CheckEdit(file, tree, wire(t, tree)); len(v) != 0 {
			t.Errorf("%s: %v", file, v)
		}
	}
}

func TestChangedPaths(t *testing.T) {
	before := treeOf(t, "a:\n  b: 1\n  c: [x]\nd: 2\n")
	after := treeOf(t, "a:\n  b: 2\n  c: [x, y]\ne: 3\n")
	got := strings.Join(changedPaths(before, after, nil), " ")
	if want := "a.b a.c e d"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
