package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TKT-3R7RF3: a view section field may override which registered widget
// renders its property, instead of the type-derived default.

// widgetConfig builds a config with one view whose single section carries the
// given widget override. sourceKnown=false makes the section reference an
// unknown collection — the RR-4ICH8M regression case, where the name check
// must still run because it needs no metamodel knowledge.
// widgetTestMetamodel is testMetamodel plus a `file`-typed property, which the
// shared fixture does not declare but the file-widget rules need.
func widgetTestMetamodel() *metamodel.Metamodel {
	meta := testMetamodel()
	td := meta.Entities["ticket"]
	td.Properties["attachment"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeFile}
	meta.Entities["ticket"] = td
	return meta
}

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
			//
			// Uses a genuine `file` property (not `title`, a string) so this
			// isolates the DISPLAY-MODE rule. With a string property the field
			// violates two rules at once and only the type error is reported —
			// which is deliberate: the type check runs first so an operator
			// isn't sent to fix the display mode only to hit a second, different
			// error on the next load.
			name:     "file widget outside a properties section is rejected",
			property: "attachment", widget: WidgetFile, display: "cards", sourceKnown: true,
			wantErr: `sets widget: file on display mode "cards"`,
		},
		{
			name:     "file widget on a properties section is accepted",
			property: "attachment", widget: WidgetFile, display: "properties", sourceKnown: true,
		},
		{
			// M1: a field violating BOTH rules reports the type error, not the
			// display-mode one — one round-trip, and the more specific message.
			name:     "type error wins over the display-mode error",
			property: "title", widget: WidgetFile, display: "cards", sourceKnown: true,
			wantErr: `widget "file" accepts: file`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := widgetConfig(tc.property, tc.widget, tc.display, tc.sourceKnown)
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, widgetTestMetamodel())
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

// widgetMetamodelWithShapes extends the shared test metamodel with the two
// property shapes whose widget rules have HIGHER precedence than the plain
// type table: a list property and an inline enum. Both were accepted
// unconditionally before RR-* below.
func widgetMetamodelWithShapes() *metamodel.Metamodel {
	meta := testMetamodel()
	td := meta.Entities["ticket"]
	td.Properties["tags"] = metamodel.PropertyDef{Type: "string", List: true}
	td.Properties["inline"] = metamodel.PropertyDef{Type: "string", Values: []string{"a", "b"}}
	meta.Entities["ticket"] = td
	return meta
}

// A list property renders through MultiSelectWidget in the SPA, whose
// defaultWidgetFor puts `list` FIRST — above values and type. Any other widget
// flattens the array through useStringValue and the auto-save then PATCHes a
// SCALAR over a list: silent data corruption on config the server called valid.
func TestValidateConfig_WidgetOnListProperty(t *testing.T) {
	tests := []struct {
		name    string
		widget  string
		wantErr bool
	}{
		{name: "multi-select is the only legal widget", widget: WidgetMultiSelect},
		{name: "textarea would flatten the list", widget: WidgetTextarea, wantErr: true},
		{name: "text would flatten the list", widget: WidgetText, wantErr: true},
		{name: "select is single-valued", widget: WidgetSelect, wantErr: true},
		{name: "checkbox is nonsense on a list", widget: WidgetCheckbox, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := widgetConfig("tags", tc.widget, "properties", true)
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, widgetMetamodelWithShapes())
			if tc.wantErr && err == nil {
				t.Fatalf("expected %q on a list property to be rejected", tc.widget)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected %q to be accepted, got: %v", tc.widget, err)
			}
			if tc.wantErr && !strings.Contains(err.Error(), "list property") {
				t.Errorf("error %q should explain the list rule", err.Error())
			}
		})
	}
}

// An inline enum (`type: string` + `values:`) routes to select in the SPA,
// which checks `values` BEFORE `type`. A free-text widget over a constrained
// set offers the operator a control that can produce invalid values.
func TestValidateConfig_WidgetOnInlineEnum(t *testing.T) {
	tests := []struct {
		name    string
		widget  string
		wantErr bool
	}{
		{name: "select matches the value set", widget: WidgetSelect},
		{name: "multi-select is allowed", widget: WidgetMultiSelect},
		{name: "text ignores the value set", widget: WidgetText, wantErr: true},
		{name: "textarea ignores the value set", widget: WidgetTextarea, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := widgetConfig("inline", tc.widget, "properties", true)
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, widgetMetamodelWithShapes())
			if tc.wantErr && err == nil {
				t.Fatalf("expected %q on an inline enum to be rejected", tc.widget)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected %q to be accepted, got: %v", tc.widget, err)
			}
		})
	}
}

// Custom types are resolved by LOOKUP in meta.Types, not by excluding a list of
// built-in names. The negation approach accepted any undeclared type name as
// enum-like and could not distinguish a value-less custom type from a real one.
func TestWidgetAcceptsProperty_CustomTypesByLookup(t *testing.T) {
	meta := testMetamodel()
	meta.Types["valueless"] = metamodel.CustomType{} // declared, but no values

	tests := []struct {
		name     string
		widget   string
		propType string
		want     bool
	}{
		{name: "declared enum type accepts select", widget: WidgetSelect, propType: "status", want: true},
		{name: "undeclared type is NOT enum-like", widget: WidgetSelect, propType: "totally-bogus"},
		{name: "value-less custom type is not select-able", widget: WidgetSelect, propType: "valueless"},
		{name: "declared enum type rejects checkbox", widget: WidgetCheckbox, propType: "status"},
		{name: "plain string accepts text", widget: WidgetText, propType: "string", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pd := metamodel.PropertyDef{Type: tc.propType}
			got, reason := widgetAcceptsProperty(tc.widget, pd, meta)
			if got != tc.want {
				t.Errorf("widgetAcceptsProperty(%q, %q) = %v (%s), want %v",
					tc.widget, tc.propType, got, reason, tc.want)
			}
		})
	}
}

// A section sourced from a multi-type traversal is LEGAL config that
// ValidateConfig does not error on, so an unvalidatable widget there would
// otherwise be accepted in total silence.
func TestCollectConfigWarnings_WidgetOnAmbiguousSource(t *testing.T) {
	meta := testMetamodel()
	// `blocks` goes ticket→ticket; make it multi-target so the collected type
	// cannot be determined statically.
	rd := meta.Relations["blocks"]
	rd.To = []string{"ticket", "category"}
	meta.Relations["blocks"] = rd

	cfg := &Config{
		Version: "1.0",
		App:     AppConfig{Name: "Test App"},
		Views: map[string]ViewConfig{
			"v": {
				Title:    "V",
				Entry:    ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{{Follow: "blocks", CollectAs: "blocked"}},
				Sections: []ViewSection{{
					Heading: "Blocked",
					Source:  "blocked",
					Display: "list",
					Fields:  []ViewSectionField{{Property: "title", Widget: WidgetTextarea}},
				}},
			},
		},
	}
	var got string
	for _, w := range CollectConfigWarnings(cfg, meta) {
		if strings.Contains(w, "several entity types") {
			got = w
			break
		}
	}
	if got == "" {
		t.Fatalf("expected an ambiguous-source warning, got %v", CollectConfigWarnings(cfg, meta))
	}
	if !strings.Contains(got, "section[0] field[0]") {
		t.Errorf("warning %q should name the precise origin", got)
	}
}

// viewCollectionTypes must agree with the map ValidateConfig builds inline.
// The "entry" collection was omitted in an earlier version, which silently
// skipped every `source: entry` section.
func TestViewCollectionTypes_IncludesEntry(t *testing.T) {
	view := ViewConfig{
		Entry:    ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{{Follow: "belongs-to", CollectAs: "cats"}},
	}
	got := viewCollectionTypes(view, testMetamodel())
	if got["entry"] != "ticket" {
		t.Errorf(`collections["entry"] = %q, want "ticket"`, got["entry"])
	}
	if got["cats"] != "category" {
		t.Errorf(`collections["cats"] = %q, want "category"`, got["cats"])
	}
}
