package dataentry

import (
	"encoding/json"
	"strings"
	"testing"

	dec "github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// refreshMeta is the Stalled probe of design section 10.1, cut down: a
// project's has_available_action reads the action roll-up held_by_tag,
// which walks to tags.
const refreshMeta = `version: "1.0"
entities:
  action:
    label: Action
    id_prefix: "ACT-"
    properties:
      status: {type: enum, values: [open, done], default: open}
      held_by_tag:
        type: boolean
        rollup:
          exists: true
          along: [tagged, tag_parent*]
          max_depth: 4
          condition: {tag: "entity.status == 'on_hold'"}
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
          condition: {action: "entity.status == 'open' and entity.held_by_tag == false"}
  tag:
    label: Tag
    id_prefix: "TAG-"
    properties:
      status: {type: enum, values: [active, on_hold]}
  note:
    label: Note
    id_prefix: "NOT-"
    properties:
      text: {type: string}
relations:
  child_of:
    from: [action]
    to: [action, project]
    max_outgoing: 1
    inverse: parent_of
  tagged:
    from: [action]
    to: [tag]
  tag_parent:
    from: [tag]
    to: [tag]
    max_outgoing: 1
`

// TestResolveListDirections_RefreshTypes pins decision Q8: the config
// response publishes, per list, the transitive set of types its roll-ups
// read, sorted, so the Stalled list refreshes on a change to a tag. A list
// of a type without roll-ups publishes none.
func TestResolveListDirections_RefreshTypes(t *testing.T) {
	meta, err := metamodel.Parse([]byte(refreshMeta))
	if err != nil {
		t.Fatal(err)
	}
	s := &Schema{Meta: meta, rollups: compileRollups(meta)}
	got := resolveListDirections(s, map[string]dec.List{
		"stalled": {EntityType: "project"},
		"notes":   {EntityType: "note"},
	})
	if types := strings.Join(got["stalled"].RefreshTypes, ","); types != "action,project,tag" {
		t.Errorf("stalled refresh_types = %s, want action,project,tag", types)
	}
	raw, err := json.Marshal(got["notes"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "refresh_types") {
		t.Errorf("notes = %s, want no refresh_types", raw)
	}
}
