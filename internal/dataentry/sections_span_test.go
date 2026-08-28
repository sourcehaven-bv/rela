package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestSectionFieldSpan_SurvivesBothConstructionSites pins the plumbing that
// carries a field's authored `span:` from config to the wire (TKT-5V8704).
//
// SectionFieldData is built in TWO places — buildSectionEntityData (cards and
// list rows) and the entry-source `properties` branch of buildSections (the
// detail page) — which were near-identical field-by-field literals. Wiring a
// new field into one and forgetting the other is the exact failure this test
// exists to catch: spans would work on the detail page and silently vanish on
// card/list sections, which reads as "spans are broken" rather than as a
// missing line in one branch.
//
// Both sites now share buildSectionFieldData, so this asserts the contract
// rather than the duplication.
func TestSectionFieldSpan_SurvivesBothConstructionSites(t *testing.T) {
	app := testViewApp()
	st := app.State()
	e := &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "First", "status": "open"},
	}
	eDef, _ := st.Meta.GetEntityDef(e.Type)

	secFields := []ViewSectionField{
		{Property: "title"},           // no span → full width
		{Property: "status", Span: 4}, // authored
	}

	t.Run("buildSectionEntityData (cards/list rows)", func(t *testing.T) {
		sed := app.views.buildSectionEntityData(context.Background(), e, secFields, eDef, "")
		assertSpans(t, sed.Fields, map[string]int{"title": 0, "status": 4})
	})

	t.Run("buildSections entry-source properties (detail page)", func(t *testing.T) {
		sections := []ViewSection{{
			Heading: "Ticket",
			Source:  "entry",
			Display: "properties",
			Fields:  secFields,
		}}
		out := app.views.buildSections(context.Background(), sections, &viewResult{Entry: e})
		if len(out) != 1 {
			t.Fatalf("buildSections: got %d sections, want 1", len(out))
		}
		assertSpans(t, out[0].Fields, map[string]int{"title": 0, "status": 4})
	})
}

// assertSpans checks each field's Span against want, keyed by property name.
// A property present in the data but missing from want is an error too — a
// silently added field would otherwise slip through.
func assertSpans(t *testing.T, fields []SectionFieldData, want map[string]int) {
	t.Helper()
	if len(fields) != len(want) {
		t.Fatalf("got %d fields, want %d", len(fields), len(want))
	}
	for _, f := range fields {
		w, ok := want[f.Property]
		if !ok {
			t.Errorf("unexpected field %q", f.Property)
			continue
		}
		if f.Span != w {
			t.Errorf("field %q: Span = %d, want %d", f.Property, f.Span, w)
		}
	}
}

// TestSectionFieldSpan_ZeroMeansFullWidth documents that an unauthored span
// stays 0 on the wire rather than being normalized to 12 here.
//
// The default lives in ONE place — the CSS `var(--field-span, 12)` fallback in
// properties-list.css. Emitting 12 from Go would duplicate it, and the two
// copies would then have to be kept in step for no benefit. `omitempty` on the
// JSON tag also keeps `span` off the wire entirely for the common case, which
// is every auto-generated view.
func TestSectionFieldSpan_ZeroMeansFullWidth(t *testing.T) {
	app := testViewApp()
	st := app.State()
	e := &entity.Entity{Type: "ticket", Properties: map[string]any{"title": "x"}}
	eDef, _ := st.Meta.GetEntityDef(e.Type)

	sed := app.views.buildSectionEntityData(
		context.Background(), e, []ViewSectionField{{Property: "title"}}, eDef, "")

	if len(sed.Fields) != 1 {
		t.Fatalf("got %d fields, want 1", len(sed.Fields))
	}
	if got := sed.Fields[0].Span; got != 0 {
		t.Errorf("unauthored span: got %d, want 0 (the CSS fallback owns the default)", got)
	}
}
