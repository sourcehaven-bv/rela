package dataentry

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestSectionEntry_MarksEntrySourcedSections pins the `entry` flag on view
// sections (TKT-VXBE16H). The SPA places a channel type with `placement:
// above_relations` before the first section without it, so the flag must
// follow `source: entry` and not the display mode: a traverse-sourced
// `content` section shows other entities and must not carry it.
func TestSectionEntry_MarksEntrySourcedSections(t *testing.T) {
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

	want := []bool{true, true, false, false}
	if len(out) != len(want) {
		t.Fatalf("buildSections: got %d sections, want %d", len(out), len(want))
	}
	for i, sec := range out {
		if sec.entry != want[i] {
			t.Errorf("section %d (%s, source %s): Entry = %v, want %v",
				i, sec.Display, sections[i].Source, sec.entry, want[i])
		}
		raw, err := json.Marshal(viewSectionToV1(sec, nil))
		if err != nil {
			t.Fatalf("marshal section %d: %v", i, err)
		}
		var wire map[string]any
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatalf("unmarshal section %d: %v", i, err)
		}
		got, present := wire["entry"]
		switch {
		case want[i] && got != true:
			t.Errorf("section %d: wire entry = %v, want true", i, got)
		case !want[i] && present:
			t.Errorf("section %d: wire carries entry = %v, want it omitted", i, got)
		}
	}
}
