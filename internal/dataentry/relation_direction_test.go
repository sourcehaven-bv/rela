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

// TestResolveConfigDirections_ListsAndKanbans pins that list columns, filter
// controls and kanban card fields get their direction materialized before the
// config reaches the SPA — the same contract resolveFormRelations gives forms.
//
// FilterBar.vue, EntityList.vue and KanbanView.vue all test the literal
// `direction === 'incoming'`, so an inferred direction that stays empty on the
// wire silently reads as outgoing in the browser.
func TestResolveConfigDirections_ListsAndKanbans(t *testing.T) {
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

	t.Run("list columns and filter controls", func(t *testing.T) {
		lists := map[string]dec.List{
			// project is the TO side, so both bindings infer incoming.
			"projects": {
				EntityType:     "project",
				Columns:        []dec.ListColumn{{Relation: "belongs-to"}, {Property: "title"}},
				FilterControls: []dec.FilterControl{{Relation: "belongs-to"}},
			},
			// task is the FROM side, so it infers outgoing.
			"tasks": {
				EntityType: "task",
				Columns:    []dec.ListColumn{{Relation: "belongs-to"}},
			},
		}
		got := resolveListDirections(s, lists)

		if d := got["projects"].Columns[0].Direction; d != dec.DirectionIncoming {
			t.Errorf("project column direction = %q, want incoming", d)
		}
		if d := got["projects"].Columns[1].Direction; d != "" {
			t.Errorf("property-only column should keep an empty direction, got %q", d)
		}
		if d := got["projects"].FilterControls[0].Direction; d != dec.DirectionIncoming {
			t.Errorf("project filter control direction = %q, want incoming", d)
		}
		if d := got["tasks"].Columns[0].Direction; d != dec.DirectionOutgoing {
			t.Errorf("task column direction = %q, want outgoing", d)
		}
		// The operator's own config must not be mutated — resolution is
		// copy-on-serve, so a later reader still sees what was authored.
		if d := lists["projects"].Columns[0].Direction; d != "" {
			t.Errorf("source config was mutated: direction = %q, want empty", d)
		}
	})

	t.Run("kanban card fields and filter controls", func(t *testing.T) {
		kanbans := map[string]dec.Kanban{
			"board": {
				EntityType:     "project",
				Card:           dec.KanbanCard{Fields: []dec.KanbanCardField{{Relation: "belongs-to"}}},
				FilterControls: []dec.FilterControl{{Relation: "belongs-to"}},
			},
		}
		got := resolveKanbanDirections(s, kanbans)

		if d := got["board"].Card.Fields[0].Direction; d != dec.DirectionIncoming {
			t.Errorf("kanban card field direction = %q, want incoming", d)
		}
		if d := got["board"].FilterControls[0].Direction; d != dec.DirectionIncoming {
			t.Errorf("kanban filter control direction = %q, want incoming", d)
		}
		if d := kanbans["board"].Card.Fields[0].Direction; d != "" {
			t.Errorf("source config was mutated: direction = %q, want empty", d)
		}
	})
}
