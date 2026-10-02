package datamigration

import (
	"slices"
	"testing"
)

func TestFile_Renames(t *testing.T) {
	to := metaV2()
	to.Entities["member"] = to.Entities["person"]
	delete(to.Entities, "person")
	rel := to.Relations["assigned-to"]
	rel.To = []string{"member"}
	to.Relations["owned-by"] = rel
	delete(to.Relations, "assigned-to")

	data := mustFileYAML(t, metaV1(), to, `  - rename_property: {entity: task, from: status, to: state}
  - map_values:
      entity: task
      property: state
      mapping: {open: todo, wip: doing}
  - rename_entity_type: {from: person, to: member}
  - rename_relation_type: {from: assigned-to, to: owned-by}
`)
	f := mustParse(t, testName("renames"), data)
	want := []Rename{
		{Kind: RenameProperty, Owner: "task", From: "status", To: "state"},
		{Kind: RenameEntityType, From: "person", To: "member"},
		{Kind: RenameRelationType, From: "assigned-to", To: "owned-by"},
	}
	if got := f.Renames(); !slices.Equal(got, want) {
		t.Errorf("Renames() = %v, want %v", got, want)
	}
}
