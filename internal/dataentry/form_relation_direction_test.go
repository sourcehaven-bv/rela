package dataentry

import (
	"testing"

	dec "github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestResolveFormRelations_Direction pins that the server resolves an absent
// `direction:` before serving form config to the SPA.
//
// This has to happen server-side: RelationCards.vue and RelationPicker.vue both
// test `props.field.direction === 'incoming'` literally, with no metamodel
// inference of their own. If the server shipped an empty direction, a to-side
// binding would validate clean and still render as outgoing in the browser —
// searching the wrong entity type and showing no existing edges.
func TestResolveFormRelations_Direction(t *testing.T) {
	mm, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  project: {label: Project, id_prefix: "PRJ-", properties: {title: {type: string}}}
  task: {label: Task, id_prefix: "TSK-", properties: {title: {type: string}}}
relations:
  belongs-to:
    label: belongs to
    from: [task]
    to: [project]
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	s := &Schema{Meta: mm}

	tests := []struct {
		name       string
		entityType string
		rel        dec.FormRelation
		want       dec.Direction
	}{
		{
			name:       "from-side binding infers outgoing",
			entityType: "task",
			rel:        dec.FormRelation{Relation: "belongs-to"},
			want:       dec.DirectionOutgoing,
		},
		{
			name:       "to-side binding infers incoming",
			entityType: "project",
			rel:        dec.FormRelation{Relation: "belongs-to"},
			want:       dec.DirectionIncoming,
		},
		{
			name:       "explicit direction is preserved",
			entityType: "task",
			rel:        dec.FormRelation{Relation: "belongs-to", Direction: dec.DirectionIncoming},
			want:       dec.DirectionIncoming,
		},
		{
			name:       "unknown relation falls back to outgoing",
			entityType: "task",
			rel:        dec.FormRelation{Relation: "no-such-relation"},
			want:       dec.DirectionOutgoing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveFormRelations(s, tt.entityType, []dec.FormRelation{tt.rel})
			if got[0].Direction != tt.want {
				t.Errorf("direction on the wire = %q, want %q", got[0].Direction, tt.want)
			}
		})
	}
}
