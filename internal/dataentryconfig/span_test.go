package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestValidateSpan covers the range rule shared by view-section fields and
// form fields (TKT-5V8704).
//
// The rule is LOUD rather than clamping. This codebase validates
// data-entry.yaml strictly at load — checkUnknownKeys even suggests
// corrections for typo'd keys — so silently rendering `span: 13` as full width
// would leave an author with a layout that ignores what they wrote and no
// diagnostic to grep for. That is the failure mode the strict validator
// exists to prevent.
func TestValidateSpan(t *testing.T) {
	tests := []struct {
		name    string
		span    Span
		wantErr bool
	}{
		{"unauthored (0) means full width, not an error", 0, false},
		{"lower bound", 1, false},
		{"a third", 4, false},
		{"a half", 6, false},
		{"upper bound", SpanColumns, false},
		{"one past the grid", SpanColumns + 1, true},
		{"far past the grid", 99, true},
		{"negative", -1, true},
		{"large negative", -12, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateSpan(tc.span, "view \"x\": section[0] field[2]")
			if tc.wantErr && len(errs) == 0 {
				t.Fatalf("validateSpan(%d): got no error, want one", tc.span)
			}
			if !tc.wantErr && len(errs) > 0 {
				t.Fatalf("validateSpan(%d): got %v, want none", tc.span, errs)
			}
			if !tc.wantErr {
				return
			}
			// The message has to locate the offending field: a config can hold
			// dozens, and "span out of range" alone would send the author
			// hunting. Every other validator in this package carries the same
			// indexed context.
			msg := errs[0]
			if !strings.Contains(msg, "section[0] field[2]") {
				t.Errorf("message must locate the field, got %q", msg)
			}
			if !strings.Contains(msg, "1-12") {
				t.Errorf("message must state the valid range, got %q", msg)
			}
		})
	}
}

// TestValidateSpan_ReportedForViewsAndForms pins that BOTH config surfaces are
// wired to the check. ViewSectionField and FormField are disjoint structs with
// separate validators, so wiring one and not the other is an easy miss — and
// would ship a span model that silently accepts garbage on forms.
func TestValidateSpan_ReportedForViewsAndForms(t *testing.T) {
	meta := testMetamodel()

	t.Run("view section field", func(t *testing.T) {
		cfg := &Config{Views: map[string]ViewConfig{
			"ticket": {
				Entry: ViewEntry{Type: "ticket"},
				Sections: []ViewSection{{
					Heading: "Ticket",
					Source:  "entry",
					Display: "properties",
					Fields:  []ViewSectionField{{Property: "title", Span: 13}},
				}},
			},
		}}
		assertSpanError(t, validateViews(cfg, meta), "view")
	})

	t.Run("form field", func(t *testing.T) {
		cfg := &Config{Forms: map[string]Form{
			"create_ticket": {
				EntityType: "ticket",
				Fields:     []FormField{{Property: "title", Span: 13}},
			},
		}}
		assertSpanError(t, validateForms(cfg, meta), "form")
	})
}

// TestValidateSpan_CheckedEvenWhenSourceUnresolvable guards a subtle placement
// bug: the view-section field loop that checks property names is nested inside
// an `if sourceType != ""` guard. Putting the span check there too would hide
// it behind an unrelated error, so an author fixing a bad `source:` would then
// discover a second, previously invisible failure.
func TestValidateSpan_CheckedEvenWhenSourceUnresolvable(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{Views: map[string]ViewConfig{
		"ticket": {
			Entry: ViewEntry{Type: "ticket"},
			Sections: []ViewSection{{
				Heading: "Broken",
				Source:  "no_such_collection", // unresolvable on purpose
				Display: "properties",
				Fields:  []ViewSectionField{{Property: "title", Span: 99}},
			}},
		},
	}}
	assertSpanError(t, validateViews(cfg, meta), "view")
}

func assertSpanError(t *testing.T, errs []string, wantPrefix string) {
	t.Helper()
	for _, e := range errs {
		if strings.Contains(e, "span") && strings.HasPrefix(e, wantPrefix) {
			return
		}
	}
	t.Errorf("no span error reported for %s; got %v", wantPrefix, errs)
}

// TestSpanRejectsFractional pins that a non-integer span is an ERROR rather
// than a silent truncation (RR-AN1J4V).
//
// yaml.v3 decodes `span: 6.5` into a plain int as 6 without complaint — it
// rejects `span: half` but truncates a float in silence. An author reaching for
// "half of a third" would get 6 columns and no diagnostic: exactly the
// layout-ignores-what-you-wrote failure the loud validation exists to prevent.
// validateSpan cannot catch it, because by then the fraction is gone.
func TestSpanRejectsFractional(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
		wantVal Span
	}{
		{"whole number decodes", "span: 6", "", 6},
		{"zero decodes", "span: 0", "", 0},
		{"fractional is rejected", "span: 6.5", "whole number of columns", 0},
		{"trailing .0 is still whole", "span: 6.0", "", 6},
		{"string is rejected", "span: half", "cannot unmarshal", 0},
		{"bool is rejected", "span: true", "cannot unmarshal", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var f FormField
			err := yaml.Unmarshal([]byte(tc.yaml), &f)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unmarshal(%q): unexpected error %v", tc.yaml, err)
				}
				if f.Span != tc.wantVal {
					t.Errorf("unmarshal(%q): Span = %d, want %d", tc.yaml, f.Span, tc.wantVal)
				}
				return
			}
			if err == nil {
				t.Fatalf("unmarshal(%q): got no error, want one mentioning %q (Span=%d)",
					tc.yaml, tc.wantErr, f.Span)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("unmarshal(%q): error %q must mention %q", tc.yaml, err, tc.wantErr)
			}
		})
	}
}

// TestSidePanelSpansAreValidated pins the hole found in review (RR-FVDPRD):
// nothing in this file descended into form.SidePanel, yet its sections carry
// ViewSectionField — Span included — and render through the same buildSections
// path as a view. A bad span there was accepted in silence.
func TestSidePanelSpansAreValidated(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{Forms: map[string]Form{
		"edit_ticket": {
			EntityType: "ticket",
			Fields:     []FormField{{Property: "title"}},
			SidePanel: &SidePanelConfig{
				Sections: []ViewSection{{
					Heading: "Details",
					Source:  "entry",
					Display: "properties",
					Fields:  []ViewSectionField{{Property: "title", Span: 999}},
				}},
			},
		},
	}}
	errs := validateForms(cfg, meta)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "side_panel") && strings.Contains(e, "span") {
			found = true
		}
	}
	if !found {
		t.Errorf("side_panel span 999 must be rejected; got %v", errs)
	}
}

// TestRelationSpanIsRejected pins that a span on a RELATION is an error rather
// than silently discarded (RR-U1B4UN).
//
// RelationCards / RelationPicker never read a span — those widgets have a
// natural minimum width. Before this, yaml.v3 dropped the unknown key without
// complaint, so `span: 6` on a relation validated clean and did nothing, which
// reads as "the feature is broken" rather than "that's not a thing here".
func TestRelationSpanIsRejected(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{Forms: map[string]Form{
		"create_ticket": {
			EntityType: "ticket",
			Relations:  []FormRelation{{Relation: "belongs-to", Span: 6}},
		},
	}}
	errs := validateForms(cfg, meta)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "cannot have a span") {
			found = true
		}
	}
	if !found {
		t.Errorf("span on a relation must be rejected; got %v", errs)
	}
}
