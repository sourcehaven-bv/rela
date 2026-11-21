package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestSectionCreate_SelfRelationOffersSameTypeInBothDirections covers a relation
// whose two ends are one type (sub-tasks: a ticket blocks a ticket). The
// affordance must offer that type, and the role of the new entity must follow
// the direction the section collected by, so the edge is not written backwards.
func TestSectionCreate_SelfRelationOffersSameTypeInBothDirections(t *testing.T) {
	tests := []struct {
		name       string
		traverse   ViewTraverse
		wantLinkAs string
	}{
		{"outgoing", ViewTraverse{From: "entry", Follow: "blocks", CollectAs: "children"}, "to"},
		{"incoming", ViewTraverse{From: "entry", FollowIncoming: "blocks", CollectAs: "children"}, "from"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			state := app.State()
			state.Cfg.Forms["create_ticket"] = dataentryconfig.Form{EntityType: "ticket"}
			state.Cfg.Views["v"] = ViewConfig{
				Entry:    ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{tc.traverse},
				Sections: []ViewSection{{
					Heading: "Sub-tasks", Source: "children", Display: "cards",
					Create: &dataentryconfig.SectionCreate{},
				}},
			}
			seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "parent"}})

			got := fetchView(context.Background(), t, app).Sections[0].Create
			if got == nil {
				t.Fatal("self-relation section with create: must carry the affordance")
			}
			if got.Relation != "blocks" || got.LinkAs != tc.wantLinkAs || got.PeerID != "TKT-001" {
				t.Errorf("affordance = %+v, want relation blocks, linkAs %q, peer TKT-001", got, tc.wantLinkAs)
			}
			if len(got.Targets) != 1 || got.Targets[0].EntityType != "ticket" {
				t.Errorf("targets = %+v, want the one same-type target", got.Targets)
			}
		})
	}
}
