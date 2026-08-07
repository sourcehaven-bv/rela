package dataentryconfig

import (
	"strings"
	"testing"
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
		span    int
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
