package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Tests for `hide_empty:` on view sections (TKT-JT101B).

// TestSectionHideEmpty_AcceptsCollectedSource is the plain case: a section on
// a collected bucket may opt in.
func TestSectionHideEmpty_AcceptsCollectedSource(t *testing.T) {
	cfg := sectionCreateConfig(nil)
	cfg.Views["ticket_detail"].Sections[0].HideEmpty = true
	if got := validateOneView(t, cfg); got != "" {
		t.Fatalf("hide_empty on a collected source must load cleanly, got: %s", got)
	}
}

// TestSectionHideEmpty_RefusesContradictions pins each combination the
// validator refuses, and that the message names the conflicting key.
func TestSectionHideEmpty_RefusesContradictions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ViewSection)
		want   string
	}{
		{
			name:   "with empty_message",
			mutate: func(s *ViewSection) { s.EmptyMessage = "Nothing here" },
			want:   "sets both hide_empty and empty_message",
		},
		{
			name:   "with create",
			mutate: func(s *ViewSection) { s.Create = &SectionCreate{} },
			want:   "sets both hide_empty and create",
		},
		{
			name: "on the entry",
			mutate: func(s *ViewSection) {
				s.Source = "entry"
				s.Display = "properties"
			},
			want: "source: entry, which is never empty",
		},
		{
			name:   "without a source",
			mutate: func(s *ViewSection) { s.Source = "" },
			want:   "but no source",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := sectionCreateConfig(nil)
			sec := &cfg.Views["ticket_detail"].Sections[0]
			sec.HideEmpty = true
			tt.mutate(sec)
			got := validateOneView(t, cfg)
			if !strings.Contains(got, tt.want) {
				t.Errorf("want an error containing %q, got: %s", tt.want, got)
			}
		})
	}
}

// TestSectionHideEmpty_RefusedInSidePanel: the side-panel path never omits
// sections, so the key is refused there rather than silently ignored.
func TestSectionHideEmpty_RefusedInSidePanel(t *testing.T) {
	cfg := &Config{
		Forms: map[string]Form{
			"ticket_form": {
				EntityType: "ticket",
				Fields:     []FormField{{Property: "title"}},
				SidePanel: &SidePanelConfig{
					Traverse: []ViewTraverse{{From: "entry", Follow: "belongs-to", CollectAs: "cats"}},
					Sections: []ViewSection{
						{Heading: "Categories", Source: "cats", Display: "cards", HideEmpty: true},
					},
				},
			},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
	if err == nil || !strings.Contains(err.Error(), "side_panel section[0] sets hide_empty") {
		t.Fatalf("want hide_empty refused in a side panel, got: %v", err)
	}
}

// TestSectionHideEmpty_DecodesFromYAML guards the struct tag: without it
// yaml.v3 would drop the key in silence and the section would never hide.
func TestSectionHideEmpty_DecodesFromYAML(t *testing.T) {
	var sec ViewSection
	if err := yaml.Unmarshal([]byte("source: cats\ndisplay: cards\nhide_empty: true\n"), &sec); err != nil {
		t.Fatal(err)
	}
	if !sec.HideEmpty {
		t.Error("hide_empty: true did not decode")
	}
}
