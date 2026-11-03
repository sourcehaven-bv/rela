package dataentry

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestSectionSource_ReachesTheWire pins the `source` field on view sections
// (TKT-VXBE16H). The SPA places a channel type with `placement:
// above_relations` before the first section whose source is not `entry`, so
// the field must carry the configured source and not the display mode: a
// collection-sourced `content` section shows other entities.
func TestSectionSource_ReachesTheWire(t *testing.T) {
	app := testViewApp()
	e := &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "First", "status": "open"},
		Content:    "Body",
	}
	sections := []ViewSection{
		{Heading: "Ticket", Source: "entry", Display: "properties", Fields: []ViewSectionField{{Property: "title"}}},
		{Source: "entry", Display: "content"},
		{Heading: "Notes", Source: "subs", Display: "content"},
		{Heading: "Subtasks", Source: "subs", Display: "cards"},
	}
	result := &viewResult{Entry: e, Collections: map[string][]*entity.Entity{"subs": {}}}
	out := app.views.buildSections(context.Background(), sections, result)

	if len(out) != len(sections) {
		t.Fatalf("buildSections: got %d sections, want %d", len(out), len(sections))
	}
	for i, sec := range out {
		raw, err := json.Marshal(viewSectionToV1(sec, nil))
		if err != nil {
			t.Fatalf("marshal section %d: %v", i, err)
		}
		var wire struct {
			Source *string `json:"source"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatalf("unmarshal section %d: %v", i, err)
		}
		if wire.Source == nil || *wire.Source != sections[i].Source {
			t.Errorf("section %d (%s): wire source = %v, want %q", i, sec.Display, wire.Source, sections[i].Source)
		}
	}
}
