package configedit

import (
	"strings"
	"testing"
)

// TestCheckEdit_Flows pins the flows.yaml allowlist: the trigger keys the
// old automations editor opened are editable, script, template, deleted and
// the flows: section are not, and a trigger holding a script keeps its `on`
// and its place among the triggers.
func TestCheckEdit_Flows(t *testing.T) {
	const flows = `triggers:
  stamp:
    description: Stamp closed tickets
    on: {entity: ticket, property: status, becomes: done}
    outputs:
      - set: {properties: {closed: "{{today}}"}}
  scripted:
    on: {entity: [ticket], created: true}
    outputs:
      - set: {properties: {state: open}}
      - script: {lua: "return true"}
flows:
  review:
    steps: [a]
`
	rule := func(tree *Map, name string) *Map {
		tr, _ := tree.Get("triggers")
		r, _ := tr.(*Map).Get(name)
		return r.(*Map)
	}
	on := func(tree *Map, name string) *Map {
		o, _ := rule(tree, name).Get("on")
		return o.(*Map)
	}
	outputs := func(tree *Map, name string) []any {
		o, _ := rule(tree, name).Get("outputs")
		return o.([]any)
	}
	output := func(key string, keys []string, values ...any) *Map {
		return &Map{Keys: []string{key}, Values: []any{&Map{Keys: keys, Values: values}}}
	}
	tests := []struct {
		name string
		edit func(tree *Map)
		want []string
	}{
		{"unchanged", func(*Map) {}, nil},
		{"edit description", func(m *Map) { rule(m, "stamp").Set("description", "x") }, nil},
		{"edit property and becomes", func(m *Map) {
			on(m, "stamp").Set("property", "state")
			on(m, "stamp").Set("becomes", "closed")
		}, nil},
		{"scalar entity to list", func(m *Map) { on(m, "stamp").Set("entity", []any{"ticket", "bug"}) }, nil},
		{"add a where clause", func(m *Map) { on(m, "stamp").Set("where", []any{"entity.kind == 'bug'"}) }, nil},
		{"add a relate output", func(m *Map) {
			rule(m, "stamp").Set("outputs", append(outputs(m, "stamp"),
				output("relate", []string{"type", "to"}, "blocks", "{{new.parent}}")))
		}, []string{"triggers.stamp.outputs.1.relate.to added", "triggers.stamp.outputs.1.relate.type added"}},
		{"add a create output", func(m *Map) {
			rule(m, "stamp").Set("outputs", append(outputs(m, "stamp"), output("create", []string{"type"}, "task")))
		}, nil},
		{"add a trigger", func(m *Map) {
			tr, _ := m.Get("triggers")
			tr.(*Map).Set("new", &Map{
				Keys: []string{"on", "outputs"},
				Values: []any{
					&Map{Keys: []string{"entity", "created"}, Values: []any{"ticket", true}},
					[]any{output("check", []string{"where", "message"}, "entity.title != ''", "needs a title")},
				},
			})
		}, nil},
		{"add a script", func(m *Map) {
			rule(m, "stamp").Set("outputs", append(outputs(m, "stamp"), output("script", []string{"lua"}, "os.exit()")))
		}, []string{"triggers.stamp.outputs.1.script.lua added"}},
		{"add a template", func(m *Map) {
			rule(m, "stamp").Set("outputs", append(outputs(m, "stamp"),
				output("create", []string{"type", "template"}, "task", "t.md")))
		}, []string{"triggers.stamp.outputs.1.create.template added"}},
		{"turn on deleted", func(m *Map) { on(m, "stamp").Set("deleted", true) },
			[]string{"triggers.stamp.on.deleted added"}},
		{"add faces", func(m *Map) { on(m, "stamp").Set("faces", "en") }, []string{"triggers.stamp.on.faces added"}},
		{"edit a flow", func(m *Map) {
			f, _ := m.Get("flows")
			r, _ := f.(*Map).Get("review")
			r.(*Map).Set("steps", []any{"b"})
		}, []string{"flows.review.steps changed"}},
		{"edit the on of a scripted trigger", func(m *Map) { on(m, "scripted").Set("updated", true) },
			[]string{"triggers.scripted.on changed"}},
		{"edit a set of a scripted trigger", func(m *Map) {
			set, _ := outputs(m, "scripted")[0].(*Map).Get("set")
			props, _ := set.(*Map).Get("properties")
			props.(*Map).Set("state", "new")
		}, nil},
		{"move the script first", func(m *Map) {
			o := outputs(m, "scripted")
			rule(m, "scripted").Set("outputs", []any{o[1], o[0]})
		}, []string{"triggers.scripted.outputs reordered"}},
		{"reorder triggers", func(m *Map) {
			tr, _ := m.Get("triggers")
			tm := tr.(*Map)
			tm.Keys[0], tm.Keys[1] = tm.Keys[1], tm.Keys[0]
			tm.Values[0], tm.Values[1] = tm.Values[1], tm.Values[0]
		}, []string{"triggers reordered"}},
		{"remove a scripted trigger", func(m *Map) {
			tr, _ := m.Get("triggers")
			tm := tr.(*Map)
			tm.Keys, tm.Values = tm.Keys[:1], tm.Values[:1]
		}, []string{"triggers.scripted removed"}},
		{"remove the output before the script", func(m *Map) {
			rule(m, "scripted").Set("outputs", outputs(m, "scripted")[1:])
		}, []string{"triggers.scripted.outputs changed"}},
		{"re-add the output after the script", func(m *Map) {
			o := outputs(m, "scripted")
			rule(m, "scripted").Set("outputs", []any{o[1], output("set", []string{"properties"},
				&Map{Keys: []string{"state"}, Values: []any{"open"}})})
		}, []string{"triggers.scripted.outputs changed"}},
		{"insert an output before the script", func(m *Map) {
			o := outputs(m, "scripted")
			rule(m, "scripted").Set("outputs", []any{o[0], output("create", []string{"type"}, "task"), o[1]})
		}, []string{"triggers.scripted.outputs changed"}},
		{"add a trigger before the others", func(m *Map) {
			tr, _ := m.Get("triggers")
			tm := tr.(*Map)
			tm.Keys = append([]string{"first"}, tm.Keys...)
			tm.Values = append([]any{&Map{
				Keys: []string{"on", "outputs"},
				Values: []any{
					&Map{Keys: []string{"entity", "created"}, Values: []any{"ticket", true}},
					[]any{output("check", []string{"message"}, "x")},
				},
			}}, tm.Values...)
		}, []string{"triggers reordered"}},
		{"replace an existing target", func(m *Map) {
			rule(m, "stamp").Set("outputs", append(outputs(m, "stamp"),
				output("create", []string{"type", "if_exists"}, "task", "replace")))
		}, []string{"triggers.stamp.outputs.1.create.if_exists added"}},
	}
	before := readTree(t, []byte(flows))
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			after := wire(t, before).(*Map)
			tc.edit(after)
			var got []string
			for _, v := range CheckEdit(FlowsFile, before, after) {
				got = append(got, v.Path+" "+v.Change)
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
