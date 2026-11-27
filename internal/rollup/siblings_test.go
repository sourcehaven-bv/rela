package rollup_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// siblingsMeta is the probe schema of design section 4.1 with one action
// property spliced in at %PROP%.
const siblingsMeta = `version: "1.0"
entities:
  action:
    label: Action
    id_prefix: "ACT-"
    properties:
      status: {type: enum, values: [open, done], default: open}
      ordering: {type: enum, values: [parallel, sequential], default: parallel}
      n: {type: integer}
      tags: {type: string, list: true}
      %PROP%
  project:
    label: Project
    id_prefix: "PRJ-"
    properties:
      ordering: {type: enum, values: [parallel, sequential], default: parallel}
  tag:
    label: Tag
    id_prefix: "TAG-"
    properties:
      status: {type: string}
relations:
  child_of:
    from: [action]
    to: [action, project]
    max_outgoing: 1
    orderable: incoming
    inverse: children
  tagged:
    from: [action]
    to: [tag]
    orderable: incoming
  in_project:
    from: [action]
    to: [project]
    max_outgoing: 1
  sorted_out:
    from: [action]
    to: [project]
    max_outgoing: 1
    orderable: outgoing
`

const waitingProp = `waiting:
        type: boolean
        rollup:
          exists: true
          siblings_before: child_of
          where: {status: open}
          when_parent: {ordering: sequential}
          climb: true
          max_depth: 12`

// TestCompile_Siblings pins the compiled probe roll-up: the relation, the
// parent and member types, the climb, and the default rule on where.
func TestCompile_Siblings(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(strings.Replace(siblingsMeta, "%PROP%", waitingProp, 1)))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	cols := set.For("action")
	if len(cols) != 1 || cols[0].Siblings == nil {
		t.Fatalf("columns = %+v", cols)
	}
	s := cols[0].Siblings
	shape := fmt.Sprintf("%s %d %v %v", s.Relation, s.Climb, s.Parents, s.Members)
	if shape != "child_of 12 [action project] [action]" {
		t.Errorf("siblings = %+v", s)
	}
	if w := s.Where["action"]; len(w) != 1 || w[0].Op != store.PropEqualOrEmpty || w[0].Value != "open" {
		t.Errorf("where = %+v, want status open or unset", s.Where)
	}
	if p := s.WhenParent["project"]; len(p) != 1 || p[0].Op != store.PropEqual || p[0].Value != "sequential" {
		t.Errorf("when_parent = %+v, want ordering sequential", s.WhenParent)
	}
	if cols[0].StoreFunc() != store.RollupCount {
		t.Errorf("StoreFunc = %v", cols[0].StoreFunc())
	}
}

// TestCompile_SiblingsRefusals pins every load-time refusal of design
// section 4.2, from the loader or from Compile.
func TestCompile_SiblingsRefusals(t *testing.T) {
	t.Parallel()
	prop := func(typ, body string) string {
		return "x:\n        type: " + typ + "\n        rollup: " + body
	}
	cases := []struct {
		name, prop, want string
	}{
		{"unknown relation", prop("boolean", `{exists: true, siblings_before: nope}`), `unknown relation "nope"`},
		{"inverse", prop("boolean", `{exists: true, siblings_before: children}`), "name the relation itself"},
		{"not ordered", prop("boolean", `{exists: true, siblings_before: in_project}`), "does not order its incoming"},
		{"ordered outgoing only", prop("boolean", `{exists: true, siblings_before: sorted_out}`),
			"does not order its incoming"},
		{"not to-one", prop("boolean", `{exists: true, siblings_before: tagged}`), "max_outgoing: 1"},
		{"climb without max_depth", prop("boolean", `{exists: true, siblings_before: child_of, climb: true}`),
			"climb needs max_depth"},
		{"climb too deep", prop("boolean", `{exists: true, siblings_before: child_of, climb: true, max_depth: 33}`),
			"climb needs max_depth, from 1 to 32"},
		{"max_depth without climb", prop("boolean", `{exists: true, siblings_before: child_of, max_depth: 3}`),
			"max_depth is only valid with climb"},
		{"climb without siblings", prop("boolean", `{exists: true, via: child_of, climb: true}`),
			"only valid with siblings_before"},
		{"count climbs", prop("integer", `{count: true, siblings_before: child_of, climb: true, max_depth: 3}`),
			"count cannot climb"},
		{"min", prop("integer", `{min: n, siblings_before: child_of}`), "folds exists or count, not min"},
		{"with via", prop("boolean", `{exists: true, siblings_before: child_of, via: child_of}`),
			"cannot be combined with via"},
		{"with along", prop("boolean", `{exists: true, siblings_before: child_of, along: [child_of]}`),
			"cannot be combined with along"},
		{"where unknown", prop("boolean", `{exists: true, siblings_before: child_of, where: {nope: x}}`),
			`where "nope" is not a property of "action"`},
		{"where bad value", prop("boolean", `{exists: true, siblings_before: child_of, where: {status: gone}}`),
			`"gone" is not one of open, done`},
		{"where list", prop("boolean", `{exists: true, siblings_before: child_of, where: {tags: a}}`),
			"is not a stored scalar"},
		{"where roll-up", prop("boolean", `{exists: true, siblings_before: child_of, where: {x: "true"}}`),
			"is not a stored scalar"},
		{"when_parent on one parent type only", prop("boolean",
			`{exists: true, siblings_before: child_of, when_parent: {status: open}}`),
			`when_parent "status" is not a property of "project"`},
		{"when_parent empty value", prop("boolean",
			`{exists: true, siblings_before: child_of, when_parent: {ordering: ""}}`),
			"has an empty property or value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			meta, err := metamodel.Parse([]byte(strings.Replace(siblingsMeta, "%PROP%", tc.prop, 1)))
			if err == nil {
				_, err = rollup.Compile(meta)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestCompile_SiblingsSourceType refuses a roll-up on a type the relation
// does not lead from.
func TestCompile_SiblingsSourceType(t *testing.T) {
	t.Parallel()
	schema := strings.Replace(siblingsMeta, "%PROP%", "", 1)
	schema = strings.Replace(schema, `      status: {type: string}`, `      status: {type: string}
      x:
        type: boolean
        rollup: {exists: true, siblings_before: child_of}`, 1)
	meta, err := metamodel.Parse([]byte(schema))
	if err == nil {
		_, err = rollup.Compile(meta)
	}
	if err == nil || !strings.Contains(err.Error(), `does not lead from "tag"`) {
		t.Errorf("error = %v", err)
	}
}
