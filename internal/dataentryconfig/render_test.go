package dataentryconfig

import (
	"strings"
	"testing"
)

// TKT-HOIX1: view section fields render as a view-oriented display value
// unless they opt in to inline edit with `render: input`.

func TestResolveFieldRender(t *testing.T) {
	tests := []struct {
		name          string
		sectionRender string
		fieldRender   string
		want          string
	}{
		{
			name: "both unset defaults to display (the breaking-change default)",
			want: RenderDisplay,
		},
		{
			name:        "field opts in",
			fieldRender: RenderInput,
			want:        RenderInput,
		},
		{
			name:          "section default applies when the field is unset",
			sectionRender: RenderInput,
			want:          RenderInput,
		},
		{
			name:          "field overrides an opted-in section",
			sectionRender: RenderInput,
			fieldRender:   RenderDisplay,
			want:          RenderDisplay,
		},
		{
			name:          "field overrides a display section",
			sectionRender: RenderDisplay,
			fieldRender:   RenderInput,
			want:          RenderInput,
		},
		{
			name:          "same value on both is a no-op",
			sectionRender: RenderInput,
			fieldRender:   RenderInput,
			want:          RenderInput,
		},
		{
			name:          "empty field render is 'inherit', not a value",
			sectionRender: RenderInput,
			fieldRender:   "",
			want:          RenderInput,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveFieldRender(tc.sectionRender, tc.fieldRender); got != tc.want {
				t.Errorf("ResolveFieldRender(%q, %q) = %q, want %q",
					tc.sectionRender, tc.fieldRender, got, tc.want)
			}
		})
	}
}

// renderConfig builds a config with one view whose single section carries the
// given render settings. sourceKnown=false makes the section reference an
// unknown collection, which is the RR-4ICH8M regression case.
func renderConfig(sectionRender, fieldRender, display string, sourceKnown bool) *Config {
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
					Render:  sectionRender,
					Fields: []ViewSectionField{
						{Property: "status", Label: "Status", Render: fieldRender},
					},
				}},
			},
		},
	}
}

func TestValidateConfig_RenderMode(t *testing.T) {
	tests := []struct {
		name        string
		section     string
		field       string
		sourceKnown bool
		wantErr     string // substring; empty means no error expected
	}{
		{name: "valid input on field", field: RenderInput, sourceKnown: true},
		{name: "valid display on field", field: RenderDisplay, sourceKnown: true},
		{name: "valid section-level render", section: RenderInput, sourceKnown: true},
		{name: "both unset", sourceKnown: true},
		{
			name:        "invalid field render",
			field:       "bogus",
			sourceKnown: true,
			wantErr:     `section[0] field[0] has invalid render mode "bogus"`,
		},
		{
			name:        "invalid section render",
			section:     "bogus",
			sourceKnown: true,
			wantErr:     `section[0] has invalid render mode "bogus"`,
		},
		{
			// RR-4ICH8M: the per-field property loops sit inside
			// source-resolution guards. `render` is a closed enum needing no
			// metamodel knowledge, so it must be validated regardless.
			name:        "invalid field render is caught when the source does not resolve",
			field:       "bogus",
			sourceKnown: false,
			wantErr:     `section[0] field[0] has invalid render mode "bogus"`,
		},
		{
			name:        "invalid section render is caught when the source does not resolve",
			section:     "bogus",
			sourceKnown: false,
			wantErr:     `section[0] has invalid render mode "bogus"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := renderConfig(tc.section, tc.field, "properties", tc.sourceKnown)
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
			if tc.wantErr == "" {
				if err != nil && strings.Contains(err.Error(), "render mode") {
					t.Fatalf("unexpected render-mode error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
			// The message must name the valid values so the operator can fix
			// it without reading the source ("config is not a secret").
			listsModes := strings.Contains(err.Error(), RenderInput) &&
				strings.Contains(err.Error(), RenderDisplay)
			if !listsModes {
				t.Errorf("error %q should list the valid render modes", err.Error())
			}
		})
	}
}

// RR-675AA0: `render` on a display mode that never renders fields is inert.
// Warn rather than error — switching a section's display mode mid-edit should
// not be a hard config-load failure.
func TestCollectConfigWarnings_InertSectionRender(t *testing.T) {
	tests := []struct {
		name     string
		display  string
		section  string
		field    string
		wantWarn bool
	}{
		{name: "properties honors render", display: "properties", section: RenderInput},
		{name: "list honors render", display: "list", field: RenderInput},
		{name: "cards honors render", display: "cards", field: RenderInput},
		{name: "table does not render fields", display: "table", section: RenderInput, wantWarn: true},
		{name: "content does not render fields", display: "content", field: RenderInput, wantWarn: true},
		{name: "table without render is silent", display: "table"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := renderConfig(tc.section, tc.field, tc.display, true)
			warnings := CollectConfigWarnings(cfg, testMetamodel())
			var got string
			for _, w := range warnings {
				if strings.Contains(w, "render:") {
					got = w
					break
				}
			}
			if tc.wantWarn && got == "" {
				t.Fatalf("expected an inert-render warning for display %q, got %v", tc.display, warnings)
			}
			if !tc.wantWarn && got != "" {
				t.Errorf("unexpected inert-render warning for display %q: %s", tc.display, got)
			}
			if tc.wantWarn && !strings.Contains(got, tc.display) {
				t.Errorf("warning %q should name the display mode %q", got, tc.display)
			}
		})
	}
}

// The warning must name the precise origin — a section-level `render:` reports
// the section, a field-level one reports the field index — so an operator with
// a long `fields:` list isn't left scanning for it.
func TestCollectConfigWarnings_InertSectionRenderNamesOrigin(t *testing.T) {
	t.Run("section-level names the section", func(t *testing.T) {
		cfg := renderConfig(RenderInput, "", "table", true)
		got := firstRenderWarning(t, cfg)
		if !strings.Contains(got, "section[0] sets render:") {
			t.Errorf("warning %q should name section[0]", got)
		}
		if strings.Contains(got, "field[") {
			t.Errorf("warning %q should not name a field when the section is the cause", got)
		}
	})

	t.Run("field-level names the field index", func(t *testing.T) {
		cfg := renderConfig("", RenderInput, "table", true)
		got := firstRenderWarning(t, cfg)
		if !strings.Contains(got, "section[0] field[0] sets render:") {
			t.Errorf("warning %q should name section[0] field[0]", got)
		}
	})
}

func firstRenderWarning(t *testing.T, cfg *Config) string {
	t.Helper()
	for _, w := range CollectConfigWarnings(cfg, testMetamodel()) {
		if strings.Contains(w, "render:") {
			return w
		}
	}
	t.Fatal("expected an inert-render warning, got none")
	return ""
}
