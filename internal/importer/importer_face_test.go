package importer

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const facedImportMetaYAML = `version: "1.0"
entities:
  policy:
    label: Policy
    id_prefix: "POL-"
    faces:
      draft: {label: Draft}
      published: {label: Published}
    properties:
      title: {type: string}
  control:
    label: Control
    id_prefix: "CTL-"
    properties:
      title: {type: string}
relations:
  implements:
    from: [policy]
    to: [control]
    scope: content
  owns:
    from: [policy]
    to: [control]
`

func facedImportMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(facedImportMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	return meta
}

// An import names a faced row as `ID@face` and a content edge's tail the
// same way; each lands on that face (BUG-J3PBFN).
func TestImport_FacedRowsAndTails(t *testing.T) {
	st := newTestStore()
	imp := New(st, facedImportMeta(t), Options{}, newTestSource())
	result, err := imp.Import(&ImportData{
		Entities: []EntityData{
			{ID: "POL-1@draft", Type: "policy", Properties: map[string]any{"title": "d"}},
			{ID: "POL-1@published", Type: "policy", Properties: map[string]any{"title": "p"}},
			{ID: "CTL-1", Type: "control", Properties: map[string]any{"title": "c"}},
		},
		Relations: []RelationData{
			{From: "POL-1@published", Relation: "implements", To: "CTL-1"},
			{From: "POL-1", Relation: "owns", To: "CTL-1"},
		},
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.EntitiesCreated != 3 || result.RelationsCreated != 2 {
		t.Fatalf("created %d entities and %d relations, want 3 and 2",
			result.EntitiesCreated, result.RelationsCreated)
	}
	for _, face := range []entity.Face{"draft", "published"} {
		if _, getErr := st.GetEntityState(ctx(), "POL-1", face); getErr != nil {
			t.Errorf("POL-1@%s: %v", face, getErr)
		}
	}
	for _, tc := range []struct {
		relType string
		face    entity.Face
	}{{"implements", "published"}, {"owns", ""}} {
		face := tc.face
		n, countErr := st.CountRelations(ctx(), store.RelationQuery{From: "POL-1", FromFace: &face, Type: tc.relType})
		if countErr != nil || n != 1 {
			t.Errorf("%s edges on tail %q = %d (%v), want 1", tc.relType, face, n, countErr)
		}
	}

	// A second import of the same content edge is a skip, not a duplicate:
	// the existence check is on the tail the edge is written to.
	again := New(st, facedImportMeta(t), Options{}, newTestSource())
	result, err = again.Import(&ImportData{Relations: []RelationData{
		{From: "POL-1@published", Relation: "implements", To: "CTL-1"},
		{From: "POL-1@draft", Relation: "implements", To: "CTL-1"},
	}})
	if err != nil {
		t.Fatalf("second Import: %v", err)
	}
	if result.RelationsSkipped != 1 || result.RelationsCreated != 1 {
		t.Errorf("second import: %d created, %d skipped; want 1 and 1",
			result.RelationsCreated, result.RelationsSkipped)
	}
}

// Rows and tails that name no declared face, or a face where none may be,
// are refused rather than written somewhere the caller did not name.
func TestImport_FaceValidation(t *testing.T) {
	seed := []EntityData{
		{ID: "POL-1@draft", Type: "policy", Properties: map[string]any{"title": "d"}},
		{ID: "CTL-1", Type: "control", Properties: map[string]any{"title": "c"}},
	}
	tests := []struct {
		name    string
		data    *ImportData
		wantErr string
	}{
		{
			name:    "bare id on a faced type",
			data:    &ImportData{Entities: []EntityData{{ID: "POL-2", Type: "policy"}}},
			wantErr: "declares faces",
		},
		{
			name:    "undeclared face",
			data:    &ImportData{Entities: []EntityData{{ID: "POL-2@archived", Type: "policy"}}},
			wantErr: `does not declare face "archived"`,
		},
		{
			name:    "face on a faceless type",
			data:    &ImportData{Entities: []EntityData{{ID: "CTL-2@draft", Type: "control"}}},
			wantErr: "declares no faces",
		},
		{
			name: "tail on an identity relation",
			data: &ImportData{Relations: []RelationData{
				{From: "POL-1@draft", Relation: "owns", To: "CTL-1"},
			}},
			wantErr: "scope: identity",
		},
		{
			name: "undeclared tail",
			data: &ImportData{Relations: []RelationData{
				{From: "POL-1@archived", Relation: "implements", To: "CTL-1"},
			}},
			wantErr: `does not declare face "archived"`,
		},
		{
			name: "undeclared tail on a source only in the batch",
			data: &ImportData{
				Entities: []EntityData{{ID: "POL-2@draft", Type: "policy", Properties: map[string]any{"title": "t"}}},
				Relations: []RelationData{
					{From: "POL-2@archived", Relation: "implements", To: "CTL-1"},
				},
			},
			wantErr: `does not declare face "archived"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newTestStore()
			if _, err := New(st, facedImportMeta(t), Options{}, newTestSource()).
				Import(&ImportData{Entities: seed}); err != nil {
				t.Fatalf("seed: %v", err)
			}
			_, err := New(st, facedImportMeta(t), Options{}, newTestSource()).Import(tt.data)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
