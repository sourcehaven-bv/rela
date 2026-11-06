package dataentry

import (
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/conflict"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// TestIdentifyConflict_SideKindMustMatchDirectory pins that a side the
// parser read as the other kind than the file's directory makes the file
// unjudgeable, so the routes answer the uniform 404 rather than judging it
// on the wrong branch.
func TestIdentifyConflict_SideKindMustMatchDirectory(t *testing.T) {
	app := newTestAppV1(t)
	root := t.TempDir()
	paths := &project.Context{
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	rel := &entity.Relation{From: "TKT-001", Type: "blocks", To: "TKT-002"}
	ent := &entity.Entity{ID: "TKT-001", Type: "ticket"}

	for _, tc := range []struct {
		name string
		path string
		side conflict.ParsedSide
	}{
		{"relation side in an entity file",
			filepath.Join(paths.EntitiesDir, "ticket", "TKT-001.md"), conflict.ParsedSide{Relation: rel}},
		{"entity side in a relation file",
			filepath.Join(paths.RelationsDir, "TKT-001--blocks--TKT-002.md"), conflict.ParsedSide{Entity: ent}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			side := tc.side
			cf := &conflict.ConflictedFile{Path: tc.path, Ours: &side, Theirs: &conflict.ParsedSide{}}
			id := identifyConflict(paths, app.State().Meta, cf)
			if len(id.rows) > 0 || len(id.rels) > 0 {
				t.Errorf("judged a mismatched file: rows=%v rels=%v", id.rows, id.rels)
			}
		})
	}
}
