package rollup_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// alongMeta is the probe schema of design section 3.1 with room for one
// more action property, spliced in at %PROP%.
const alongMeta = `version: "1.0"
entities:
  action:
    label: Action
    id_prefix: "ACT-"
    properties:
      status: {type: enum, values: [open, done, dropped], default: open}
      due: {type: date}
      n: {type: integer}
      %PROP%
  project:
    label: Project
    id_prefix: "PRJ-"
    properties:
      status: {type: enum, values: [active, on_hold, dropped], default: active}
      due: {type: date}
      n: {type: date}
      label2:
        type: integer
        computed: entity.size * 2
      size: {type: integer}
  folder:
    label: Folder
    id_prefix: "FLD-"
    properties:
      status: {type: enum, values: [active, dropped]}
  tag:
    label: Tag
    id_prefix: "TAG-"
    properties:
      status: {type: enum, values: [active, on_hold]}
relations:
  child_of:
    from: [action]
    to: [action, project]
    max_outgoing: 1
    inverse: parent_of
  in_folder:
    from: [project, folder]
    to: [folder]
    max_outgoing: 1
  tagged:
    from: [action]
    to: [tag]
  tag_parent:
    from: [tag]
    to: [tag]
    max_outgoing: 1
  tag_peer:
    from: [tag]
    to: [tag]
`

func alongSchema(prop string) string {
	return strings.Replace(alongMeta, "%PROP%", prop, 1)
}

const heldProp = `held:
        type: boolean
        rollup:
          exists: true
          along: [child_of*, in_folder*]
          max_depth: 12
          condition:
            action: "entity.status ~= 'open'"
            project: "entity.status == 'on_hold' or entity.status == 'dropped'"
            folder: "entity.status == 'dropped'"`

// TestCompile_Along pins the compiled walk of the probe's held roll-up: the
// segments, the types each reaches, and a lowered node condition per type.
func TestCompile_Along(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(alongSchema(heldProp)))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	cols := set.For("action")
	if len(cols) != 1 || cols[0].Along == nil {
		t.Fatalf("columns = %+v", cols)
	}
	a := cols[0].Along
	if len(a.Segments) != 2 || a.Segments[0].Relation != "child_of" || !a.Segments[0].Repeat ||
		strings.Join(a.Segments[0].Reaches, ",") != "action,project" ||
		strings.Join(a.Segments[1].Reaches, ",") != "folder" || a.MaxDepth != 12 {
		t.Errorf("segments = %+v", a.Segments)
	}
	if !a.Lowers() || len(a.Nodes) != 3 {
		t.Errorf("nodes = %+v, lowers = %v", a.Nodes, a.Lowers())
	}
	if cols[0].StoreFunc() != store.RollupCount {
		t.Errorf("StoreFunc = %v", cols[0].StoreFunc())
	}
}

// TestCompile_AlongDefaults pins the default fill: a project without a
// status holds the default active, in Go and in the store form.
func TestCompile_AlongDefaults(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(alongSchema(`active_above:
        type: boolean
        rollup:
          exists: true
          along: [child_of*]
          max_depth: 4
          condition:
            project: "entity.status == 'active'"`)))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	node := set.For("action")[0].Along.Nodes["project"]
	ctx := context.Background()
	for _, tc := range []struct {
		props  map[string]any
		hidden []string
		want   bool
	}{
		{nil, nil, true},
		{map[string]any{"status": ""}, nil, true},
		{map[string]any{"status": "on_hold"}, nil, false},
		{nil, []string{"status"}, false},
	} {
		got, err := node.Holds(ctx, "PRJ-1", tc.props, tc.hidden)
		if err != nil || got != tc.want {
			t.Errorf("Holds(%v, hidden %v) = %v, %v; want %v", tc.props, tc.hidden, got, err, tc.want)
		}
	}
	filled, err := node.FillDefaults(*node.Lowered)
	if err != nil {
		t.Fatal(err)
	}
	if filled.Op != store.CondOr {
		t.Errorf("filled = %+v, want the leaf ORed with an empty test", filled)
	}
}

// TestCompile_AlongRefusals pins every load-time refusal of design section
// 3.2. Each case is one action property; the error comes from the loader
// or from Compile, whichever checks it.
func TestCompile_AlongRefusals(t *testing.T) {
	t.Parallel()
	rollupProp := func(typ, body string) string {
		return "x:\n        type: " + typ + "\n        rollup: " + body
	}
	cases := []struct {
		name, prop, want string
	}{
		{"via", rollupProp("boolean",
			`{exists: true, via: tagged, along: [tagged], max_depth: 2, condition: {tag: "entity.status == 'active'"}}`),
			"along cannot be combined with via"},
		{"count", rollupProp("integer",
			`{count: true, along: [tagged], max_depth: 2, condition: {tag: "entity.status == 'active'"}}`),
			"along folds exists, min or max, not count"},
		{"sum", rollupProp("integer",
			`{sum: n, along: [child_of*], max_depth: 2, condition: {action: "entity.status == 'open'"}}`),
			"along folds exists, min or max, not sum"},
		{"latest_by", rollupProp("string",
			`{latest_by: due, pick: status, along: [child_of*], max_depth: 2, condition: {action: "true"}}`),
			"along cannot be combined with pick"},
		{"unknown relation", rollupProp("boolean",
			`{exists: true, along: [nope], max_depth: 2, condition: {tag: "true"}}`),
			`unknown relation "nope"`},
		{"near side", rollupProp("boolean",
			`{exists: true, along: [tag_parent*], max_depth: 2, condition: {tag: "true"}}`),
			"accepts none of action"},
		{"repeat not back to the same type", rollupProp("boolean",
			`{exists: true, along: [tagged*], max_depth: 2, condition: {tag: "true"}}`),
			"lead from a type back to the same type"},
		{"repeat not to-one", rollupProp("boolean",
			`{exists: true, along: [tagged, tag_peer*], max_depth: 2, condition: {tag: "true"}}`),
			"declares to-one"},
		{"inverse repeat not to-one", rollupProp("boolean",
			`{exists: true, along: [parent_of*], max_depth: 2, condition: {action: "true"}}`),
			"declares to-one"},
		{"max_depth missing", rollupProp("boolean",
			`{exists: true, along: [tagged], condition: {tag: "true"}}`),
			"along needs max_depth"},
		{"max_depth below 1", rollupProp("boolean",
			`{exists: true, along: [tagged], max_depth: -1, condition: {tag: "true"}}`),
			"max_depth -1 is outside 1 to 32"},
		{"max_depth above 32", rollupProp("boolean",
			`{exists: true, along: [tagged], max_depth: 33, condition: {tag: "true"}}`),
			"max_depth 33 is outside 1 to 32"},
		{"condition type not reached", rollupProp("boolean",
			`{exists: true, along: [tagged], max_depth: 2, condition: {folder: "true"}}`),
			`condition type "folder" is not a type the walk reaches`},
		{"condition does not compile", rollupProp("boolean",
			`{exists: true, along: [tagged], max_depth: 2, condition: {tag: "entity.status =="}}`),
			"does not compile"},
		{"condition reads a roll-up", rollupProp("boolean",
			`{exists: true, along: [child_of*], max_depth: 2, condition: {action: "entity.x == true"}}`),
			`reads the roll-up "x"`},
		{"condition reads a computed property", rollupProp("boolean",
			`{exists: true, along: [child_of*], max_depth: 2, condition: {project: "entity.label2 == 4"}}`),
			`reads the computed property "label2"`},
		{"condition reads current_user", rollupProp("boolean",
			`{exists: true, along: [tagged], max_depth: 2, condition: {tag: "entity.status == current_user.id"}}`),
			"reads current_user"},
		{"condition calls has_role", rollupProp("boolean",
			`{exists: true, along: [tagged], max_depth: 2, condition: {tag: "has_role('admin')"}}`),
			"does not compile"},
		{"condition calls related", rollupProp("boolean",
			`{exists: true, along: [tagged], max_depth: 2, condition: {tag: "related(entity, 'tag_parent')"}}`),
			"related()"},
		{"condition calls a graph function", rollupProp("boolean",
			`{exists: true, along: [tagged], max_depth: 2, condition: {tag: "has_relation(entity, 'tag_parent')"}}`),
			"does not compile"},
		{"no condition", rollupProp("boolean", `{exists: true, along: [tagged], max_depth: 2}`),
			"along needs a condition"},
		{"min property type differs", rollupProp("integer",
			`{min: n, along: [child_of*], max_depth: 2, condition: {action: "true", project: "true"}}`),
			"differs across visited types"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			meta, err := metamodel.Parse([]byte(alongSchema(tc.prop)))
			if err == nil {
				_, err = rollup.Compile(meta)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
