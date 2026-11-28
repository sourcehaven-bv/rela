package rollup_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
)

// nestedMeta is the probe schema of design section 10.1: a project's
// has_available_action reads the action roll-ups held and held_by_tag.
// %ACTION% and %PROJECT% splice in more properties.
const nestedMeta = `version: "1.0"
entities:
  action:
    label: Action
    id_prefix: "ACT-"
    properties:
      status: {type: enum, values: [open, done], default: open}
      held:
        type: boolean
        rollup:
          exists: true
          along: [child_of*]
          max_depth: 4
          condition:
            action: "entity.status ~= 'open'"
            project: "entity.status == 'on_hold'"
      held_by_tag:
        type: boolean
        rollup:
          exists: true
          along: [tagged, tag_parent*]
          max_depth: 4
          condition: {tag: "entity.status == 'on_hold'"}
      %ACTION%
  project:
    label: Project
    id_prefix: "PRJ-"
    properties:
      status: {type: enum, values: [active, on_hold], default: active}
      has_available_action:
        type: boolean
        rollup:
          exists: true
          via: parent_of
          condition:
            action: "entity.status == 'open' and entity.held == false and entity.held_by_tag == false"
      %PROJECT%
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
    from: [project]
    to: [folder]
    max_outgoing: 1
  tagged:
    from: [action]
    to: [tag]
  tag_parent:
    from: [tag]
    to: [tag]
    max_outgoing: 1
`

func nestedSchema(action, project string) string {
	return strings.NewReplacer("%ACTION%", action, "%PROJECT%", project).Replace(nestedMeta)
}

func compileNested(t *testing.T, action, project string) (*rollup.Set, error) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(nestedSchema(action, project)))
	if err != nil {
		t.Fatal(err)
	}
	return rollup.Compile(meta)
}

// TestCompile_Nested pins the outer roll-up of section 10.1: a via with
// condition compiles to a one-hop walk whose action node reads the two
// inner roll-ups and has no store form.
func TestCompile_Nested(t *testing.T) {
	t.Parallel()
	set, err := compileNested(t, "", "")
	if err != nil {
		t.Fatal(err)
	}
	var col rollup.Column
	for _, c := range set.For("project") {
		if c.Property == "has_available_action" {
			col = c
		}
	}
	a := col.Along
	if a == nil || len(a.Segments) != 1 || a.Segments[0].Relation != "child_of" || !a.Segments[0].SourceIsTo ||
		a.Segments[0].Repeat || a.MaxDepth != 1 {

		t.Fatalf("along = %+v", a)
	}
	node := a.Nodes["action"]
	if node == nil || strings.Join(node.Rollups, ",") != "held,held_by_tag" || node.Lowered != nil || a.Lowers() {
		t.Fatalf("node = %+v", node)
	}
	got := col.Inner()
	if len(got) != 2 || got[0].String() != "action.held" || got[1].String() != "action.held_by_tag" {
		t.Errorf("Inner = %v", got)
	}
}

// TestCompile_NestedRefusals pins the load rules of section 10.2: each
// refusal names the chain or cycle it found.
func TestCompile_NestedRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, action, project, want string
	}{
		{"two levels", `outer2:
        type: boolean
        rollup:
          exists: true
          along: [child_of]
          max_depth: 1
          condition: {project: "entity.has_available_action == true"}`, "",
			"more than one level deep: action.outer2 -> project.has_available_action -> action.held"},
		{"reads itself", `self:
        type: boolean
        rollup:
          exists: true
          along: [child_of*]
          max_depth: 2
          condition: {action: "entity.self == true"}`, "",
			"cycle: action.self -> action.self"},
		{"two roll-up cycle", `a1:
        type: boolean
        rollup:
          exists: true
          along: [child_of]
          max_depth: 1
          condition: {project: "entity.p1 == true"}`, `p1:
        type: boolean
        rollup:
          exists: true
          via: parent_of
          condition: {action: "entity.a1 == true"}`,
			"cycle: action.a1 -> project.p1 -> action.a1"},
		{"inner not declared on the named type", "", `bad:
        type: boolean
        rollup:
          exists: true
          via: parent_of
          condition: {action: "entity.missing == true"}`,
			`property "bad": rollup: condition for "action": does not compile`},
		{"via condition over two relations", "", `bad:
        type: boolean
        rollup:
          exists: true
          via: [in_folder, in_folder]
          condition: {folder: "true"}`,
			"a via with condition names exactly one relation"},
		{"via condition with sum", "", `bad:
        type: integer
        rollup:
          sum: status
          via: parent_of
          condition: {action: "true"}`,
			"a via with condition folds exists, count, min or max, not sum"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			meta, err := metamodel.Parse([]byte(nestedSchema(tc.action, tc.project)))
			if err == nil {
				_, err = rollup.Compile(meta)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestSet_ReadTypes pins the refresh set of decision Q8: the project list
// reads the types of has_available_action and, through it, of held and
// held_by_tag, so a change to a tag refreshes it.
func TestSet_ReadTypes(t *testing.T) {
	t.Parallel()
	set, err := compileNested(t, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(set.ReadTypes("project", readsAll), ","); got != "action,project,tag" {
		t.Errorf("project ReadTypes = %s", got)
	}
	if got := strings.Join(set.ReadTypes("action", readsAll), ","); got != "action,project,tag" {
		t.Errorf("action ReadTypes = %s", got)
	}
	if got := set.ReadTypes("folder", readsAll); len(got) != 0 {
		t.Errorf("folder ReadTypes = %v", got)
	}
	var nilSet *rollup.Set
	if got := nilSet.ReadTypes("project", readsAll); got != nil {
		t.Errorf("nil set ReadTypes = %v", got)
	}
}

func readsAll(string) bool { return true }

// TestSet_ReadTypesOnlyReadRollups pins that only the roll-ups a view reads
// add types: held alone reads action and project, never tag, and a view
// that reads only stored properties refreshes on nothing.
func TestSet_ReadTypesOnlyReadRollups(t *testing.T) {
	t.Parallel()
	set, err := compileNested(t, "note: {type: string}", "")
	if err != nil {
		t.Fatal(err)
	}
	stored := func(p string) bool { return p == "note" || p == "status" }
	if got := set.ReadTypes("action", stored); len(got) != 0 {
		t.Errorf("action ReadTypes(stored) = %v, want none", got)
	}
	held := func(p string) bool { return p == "held" }
	if got := strings.Join(set.ReadTypes("action", held), ","); got != "action,project" {
		t.Errorf("action ReadTypes(held) = %s, want action,project", got)
	}
	none := func(string) bool { return false }
	if got := set.ReadTypes("project", none); len(got) != 0 {
		t.Errorf("project ReadTypes(none) = %v, want none", got)
	}
}

// TestSet_ReadTypesTransitive pins that the inner roll-up's types join the
// outer's even when the outer walk itself never reaches them: here only
// held_by_tag reads tag.
func TestSet_ReadTypesTransitive(t *testing.T) {
	t.Parallel()
	set, err := compileNested(t, "", `tagless:
        type: boolean
        rollup:
          exists: true
          via: parent_of
          condition: {action: "entity.held_by_tag == false"}`)
	if err != nil {
		t.Fatal(err)
	}
	var col rollup.Column
	for _, c := range set.For("project") {
		if c.Property == "tagless" {
			col = c
		}
	}
	if got := strings.Join(col.Types, ","); got != "action,project" {
		t.Errorf("direct Types = %s", got)
	}
	if got := strings.Join(set.ReadTypes("project", readsAll), ","); !strings.Contains(got, "tag") {
		t.Errorf("ReadTypes = %s, want tag through held_by_tag", got)
	}
}
