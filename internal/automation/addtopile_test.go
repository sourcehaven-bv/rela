package automation

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestEngine_AddToPilePlansPush(t *testing.T) {
	no := false
	engine, err := NewEngineFromMetamodel(nil, []metamodel.AutomationDef{{
		Name: "inbox",
		On:   metamodel.AutomationTrigger{Entity: metamodel.StringOrSlice{"ticket"}, Created: true},
		Do: []metamodel.AutomationAction{
			{AddToPile: &metamodel.AddToPileAction{Pile: "Inbox", Owner: "{{new.assignee}}"}},
			{AddToPile: &metamodel.AddToPileAction{Pile: "{{new.queue}}", Create: &no}},
		},
	}})
	if err != nil {
		t.Fatalf("build engine: %v", err)
	}

	for _, tc := range []struct {
		name  string
		props map[string]any
		want  []PileToPush
		warns int
	}{
		{
			name:  "both interpolate",
			props: map[string]any{"assignee": "PER-1", "queue": "Triage"},
			want: []PileToPush{
				{Pile: "Inbox", Owner: "PER-1", Create: true, AutomationName: "inbox"},
				{Pile: "Triage", Owner: "", Create: false, AutomationName: "inbox"},
			},
		},
		{
			// An owner template that resolves to nothing must not fall back
			// to the acting user; an empty pile name is a warning.
			name:  "unassigned and no queue",
			props: map[string]any{},
			want:  nil,
			warns: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ent := &entity.Entity{ID: "TKT-1", Type: "ticket", Face: "draft", Properties: tc.props}
			res := engine.Process(context.Background(), Event{Type: EventEntityCreated, Entity: ent})
			if len(res.PilesToPush) != len(tc.want) {
				t.Fatalf("PilesToPush = %+v, want %+v", res.PilesToPush, tc.want)
			}
			for i := range tc.want {
				if res.PilesToPush[i] != tc.want[i] {
					t.Errorf("PilesToPush[%d] = %+v, want %+v", i, res.PilesToPush[i], tc.want[i])
				}
			}
			if len(res.Warnings) != tc.warns {
				t.Errorf("warnings = %v, want %d", res.Warnings, tc.warns)
			}
		})
	}
}
