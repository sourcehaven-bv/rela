package dataentryconfig

import (
	"strings"
	"testing"
)

// TKT-3R7RF3: a view section field may override which registered widget
// renders its property, instead of the type-derived default.

// widgetConfig builds a config with one view whose single section carries the
// given widget override. sourceKnown=false makes the section reference an
// unknown collection — the RR-4ICH8M regression case, where the name check
// must still run because it needs no metamodel knowledge.
func widgetConfig(property, widget, display string, sourceKnown bool) *Config {
	source := "entry"
	if !sourceKnown {
		source = "no_such_collection"
	}
	return &Config{
		Version: "1.0",
		App:     AppConfig{Name: "Test App"},
		Views: map[string]ViewConfig{
			"ticket_view": {
				Title: "Ticket",
				Entry: ViewEntry{Type: "ticket"},
				Sections: []ViewSection{{
					Heading: "Details",
					Source:  source,
					Display: display,
					Fields: []ViewSectionField{
						{Property: property, Label: "L", Widget: widget},
					},
				}},
			},
		},
	}
}

func TestValidateConfig_SectionFieldWidget(t *testing.T) {
	tests := []struct {
		name        string
		property    string
		widget      string
		display     string
		sourceKnown bool
		wantErr     string // substring; empty means no widget error expected
	}{
		{name: "no widget is the default", property: "status", display: "properties", sourceKnown: true},
		{
			name: "select on an enum property", property: "status", widget: WidgetSelect,
			display: "properties", sourceKnown: true,
		},
		{
			name:     "textarea on a string property is the motivating override",
			property: "title", widget: WidgetTextarea, display: "properties", sourceKnown: true,
		},
		{
			name: "unregistered name is rejected", property: "status", widget: "bogus",
			display: "properties", sourceKnown: true,
			wantErr: `section[0] field[0] has invalid widget "bogus"`,
		},
		{
			// The name check needs no metamodel knowledge, so an unresolvable
			// source must not suppress it (RR-4ICH8M).
			name:     "unregistered name is caught when the source does not resolve",
			property: "status", widget: "bogus", display: "properties", sourceKnown: false,
			wantErr: `section[0] field[0] has invalid widget "bogus"`,
		},
		{
			name:     "whitespace-only is rejected, not trimmed to empty",
			property: "status", widget: "  ", display: "properties", sourceKnown: true,
			wantErr: `has invalid widget "  "`,
		},
		{
			// Widget names are lowercase throughout; a silent case-fold would
			// make config non-canonical, and the SPA does not fold either.
			name: "capitalised name is rejected", property: "status", widget: "Checkbox",
			display: "properties", sourceKnown: true,
			wantErr: `has invalid widget "Checkbox"`,
		},
		{
			name:     "checkbox on a string property is a type mismatch",
			property: "title", widget: WidgetCheckbox, display: "properties", sourceKnown: true,
			wantErr: `sets widget "checkbox" on property "title" of type "string"`,
		},
		{
			name:     "date widget on a string property is a type mismatch",
			property: "title", widget: WidgetDate, display: "properties", sourceKnown: true,
			wantErr: `sets widget "date" on property "title"`,
		},
		{
			// RR-NGY84F: only the entry mount site passes :attachments, so a
			// FileWidget in a cards/list row would render with none.
			name:     "file widget outside a properties section is rejected",
			property: "title", widget: WidgetFile, display: "cards", sourceKnown: true,
			wantErr: `sets widget: file on display mode "cards"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := widgetConfig(tc.property, tc.widget, tc.display, tc.sourceKnown)
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
			if tc.wantErr == "" {
				if err != nil && strings.Contains(err.Error(), "widget") {
					t.Fatalf("unexpected widget error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// The invalid-name message must list the valid set so the operator can fix it
// without reading the source — the config is not a secret (CLAUDE.md).
func TestValidateConfig_SectionFieldWidget_ErrorListsValidNames(t *testing.T) {
	cfg := widgetConfig("status", "bogus", "properties", true)
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{WidgetCheckbox, WidgetSelect, WidgetTextarea, WidgetFile} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should list valid widget %q", err.Error(), want)
		}
	}
}

// A type-mismatch message must name what the widget DOES accept, otherwise the
// operator has to guess which widget to reach for instead.
func TestValidateConfig_SectionFieldWidget_MismatchNamesAcceptedTypes(t *testing.T) {
	cfg := widgetConfig("title", WidgetCheckbox, "properties", true)
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "accepts: boolean") {
		t.Errorf("error %q should name the accepted types", err.Error())
	}
}

// RR-2GBB0V / AC5: a widget on a property the metamodel does not declare is
// ignored at render (the SPA routes it through resolveFromHint, which takes no
// name), so warn rather than error — the operator otherwise gets silence.
func TestCollectConfigWarnings_WidgetOnUndeclaredProperty(t *testing.T) {
	cfg := widgetConfig("not_a_real_property", WidgetTextarea, "properties", true)
	got := firstWidgetWarning(t, cfg)
	if !strings.Contains(got, "section[0] field[0]") {
		t.Errorf("warning %q should name the precise origin", got)
	}
	if !strings.Contains(got, "not_a_real_property") {
		t.Errorf("warning %q should name the property", got)
	}
	// It must not ALSO be a hard error — that is the whole point of warning.
	if err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel()); err != nil {
		if strings.Contains(err.Error(), "widget") {
			t.Errorf("undeclared property should warn, not error: %v", err)
		}
	}
}

// RR-675AA0's sibling: `widget:` on a display mode that renders no fields at
// all is inert, same as `render:` is.
func TestCollectConfigWarnings_InertWidget(t *testing.T) {
	tests := []struct {
		name     string
		display  string
		wantWarn bool
	}{
		{name: "properties renders fields", display: "properties"},
		{name: "list renders fields", display: "list"},
		{name: "cards renders fields", display: "cards"},
		{name: "table does not render fields", display: "table", wantWarn: true},
		{name: "content does not render fields", display: "content", wantWarn: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Use a declared property so only the display mode can warn.
			cfg := widgetConfig("status", WidgetSelect, tc.display, true)
			var got string
			for _, w := range CollectConfigWarnings(cfg, testMetamodel()) {
				if strings.Contains(w, "widget") {
					got = w
					break
				}
			}
			if tc.wantWarn && got == "" {
				t.Fatalf("expected an inert-widget warning for display %q", tc.display)
			}
			if !tc.wantWarn && got != "" {
				t.Errorf("unexpected inert-widget warning for display %q: %s", tc.display, got)
			}
			if tc.wantWarn && !strings.Contains(got, tc.display) {
				t.Errorf("warning %q should name the display mode %q", got, tc.display)
			}
		})
	}
}

// A section with no widget authored must stay completely silent — a warning on
// every ordinary section would train operators to ignore the channel.
func TestCollectConfigWarnings_NoWidgetIsSilent(t *testing.T) {
	cfg := widgetConfig("status", "", "table", true)
	for _, w := range CollectConfigWarnings(cfg, testMetamodel()) {
		if strings.Contains(w, "widget") {
			t.Errorf("unexpected widget warning when none is authored: %s", w)
		}
	}
}

func firstWidgetWarning(t *testing.T, cfg *Config) string {
	t.Helper()
	for _, w := range CollectConfigWarnings(cfg, testMetamodel()) {
		if strings.Contains(w, "widget") {
			return w
		}
	}
	t.Fatal("expected a widget warning, got none")
	return ""
}
