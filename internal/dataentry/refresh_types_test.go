package dataentry

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
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
// response publishes, per list, the transitive set of types the roll-ups it
// reads read, sorted, so the Stalled list refreshes on a change to a tag. A
// roll-up counts as read through a column, a sort, a filter, a filter
// control or the condition. A list that reads none, even over a type with
// roll-ups, publishes none.
func TestResolveListDirections_RefreshTypes(t *testing.T) {
	meta, err := metamodel.Parse([]byte(refreshMeta))
	if err != nil {
		t.Fatal(err)
	}
	s := &Schema{Meta: meta, rollups: compileRollups(meta)}
	const want = "action,project,tag"
	condReads := func(id string) map[string]bool {
		if id == "by_condition" {
			return map[string]bool{"has_available_action": true}
		}
		return nil
	}
	got := resolveListDirections(s, map[string]dec.List{
		"by_column":    {EntityType: "project", Columns: []dec.ListColumn{{Property: "has_available_action"}}},
		"by_sort":      {EntityType: "project", Sort: []dec.SortSpec{{Property: "has_available_action"}}},
		"by_filter":    {EntityType: "project", Filters: []dec.FilterConfig{{Property: "has_available_action"}}},
		"by_control":   {EntityType: "project", FilterControls: []dec.FilterControl{{Property: "has_available_action"}}},
		"by_condition": {EntityType: "project"},
		"plain":        {EntityType: "project", Columns: []dec.ListColumn{{Property: "status"}}},
		"notes":        {EntityType: "note"},
	}, condReads)
	for _, id := range []string{"by_column", "by_sort", "by_filter", "by_control", "by_condition"} {
		if types := strings.Join(got[id].RefreshTypes, ","); types != want {
			t.Errorf("%s refresh_types = %s, want %s", id, types, want)
		}
	}
	for _, id := range []string{"plain", "notes"} {
		raw, err := json.Marshal(got[id])
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "refresh_types") {
			t.Errorf("%s = %s, want no refresh_types", id, raw)
		}
	}
}

// TestListConditionReads_RealCompiler pins that the roll-ups a list's
// condition reads come from the compiled condition: only the one it names.
func TestListConditionReads_RealCompiler(t *testing.T) {
	meta, err := metamodel.Parse([]byte(refreshMeta))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &dec.Config{Lists: map[string]dec.List{
		"stalled": {EntityType: "project", Condition: "entity.has_available_action == false"},
		"active":  {EntityType: "project", Condition: "entity.status == 'active'"},
	}}
	s := &Schema{Meta: meta, Cfg: cfg, rollups: compileRollups(meta)}
	fn := withConditionRollups(AdaptViewConditions(appbuild.ViewConditions), rollupSource{})
	h := &schemaHandler{viewConditions: func() ViewConditionFunc { return fn }}
	reads := h.listConditionReads(s)
	if reads == nil {
		t.Fatal("listConditionReads = nil, want a lookup")
	}
	if got := reads("stalled"); len(got) != 1 || !got["has_available_action"] {
		t.Errorf("stalled reads %v, want has_available_action", got)
	}
	if got := reads("active"); len(got) != 0 {
		t.Errorf("active reads %v, want none", got)
	}
	served := resolveListDirections(s, cfg.Lists, reads)
	if types := strings.Join(served["stalled"].RefreshTypes, ","); types != "action,project,tag" {
		t.Errorf("stalled refresh_types = %s", types)
	}
	if types := served["active"].RefreshTypes; len(types) != 0 {
		t.Errorf("active refresh_types = %v, want none", types)
	}
}
