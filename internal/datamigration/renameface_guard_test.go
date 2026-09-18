package datamigration

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// A rename into a type whose declared faces differ strands every row whose
// face the destination does not declare: the rows keep their coordinate and
// the new type names nothing at it, so nothing can reach them. Validate holds
// both shapes and both carry Faces, so it can refuse this before any write.
func TestRenameEntityType_RefusesDivergentFaceSets(t *testing.T) {
	// from: `task` declares draft/published. to: `job` declares neither.
	from := metaV1()
	def := from.Entities["task"]
	def.Faces = map[string]metamodel.FaceDef{"draft": {}, "published": {}}
	from.Entities["task"] = def

	to := metaV1()
	job := to.Entities["task"]
	job.Faces = map[string]metamodel.FaceDef{"live": {}}
	delete(to.Entities, "task")
	to.Entities["job"] = job

	step := &renameEntityTypeStep{From: "task", To: "job"}
	err := step.Validate(from.ShapeProjection(), to.ShapeProjection())
	if err == nil {
		t.Fatal("renaming into a type declaring a different face set must be refused: " +
			"every draft/published row would keep a coordinate `job` does not declare")
	}
	for _, want := range []string{"draft", "published", "job"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name %q so the operator can act: %v", want, err)
		}
	}
}

// `rela migrate gen` must still produce a usable draft for a faced rename.
//
// The rename guess comes from propertyShapesSimilar, which compares PROPERTIES
// only, so it pairs two types whose face sets diverge. Emitting a live
// rename_entity_type there produces a draft that fails its own validation, and
// Generate's round-trip self-check turns that into "generated draft does not
// parse (generator bug)" with NO FILE AT ALL — the tool blaming itself for a
// schema change the operator is entitled to make.
func TestGenerate_FacedRenameDraftsACommentedStep(t *testing.T) {
	from := metaV1()
	def := from.Entities["task"]
	def.Faces = map[string]metamodel.FaceDef{"draft": {}, "published": {}}
	from.Entities["task"] = def

	to := metaV1()
	job := to.Entities["task"]
	job.Faces = nil
	delete(to.Entities, "task")
	to.Entities["job"] = job

	d, err := Generate(from.ShapeProjection(), to.ShapeProjection(), "faced rename", testNow())
	if err != nil {
		t.Fatalf("a faced rename must still draft, not fail as a generator bug: %v", err)
	}
	body := string(d.Content)
	// Commented, so the draft parses; and the TODO names the faces and the
	// remedy, so the operator knows what to do before uncommenting.
	if !strings.Contains(body, "# - rename_entity_type: {from: task, to: job}") {
		t.Errorf("the rename should be drafted COMMENTED; got:\n%s", body)
	}
	for _, want := range []string{"draft, published", "rename_face", "migrate_face"} {
		if !strings.Contains(body, want) {
			t.Errorf("the TODO should mention %q so the operator can act; got:\n%s", want, body)
		}
	}
}
