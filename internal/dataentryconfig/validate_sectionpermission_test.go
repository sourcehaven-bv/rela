package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Tests for `permission:` on view sections.

// TestSectionPermission_Config pins that the key decodes, loads on a view
// section, and is refused on a side-panel section, which never omits one.
func TestSectionPermission_Config(t *testing.T) {
	var sec ViewSection
	if err := yaml.Unmarshal([]byte("source: entry\ndisplay: content\npermission: staff.manage\n"), &sec); err != nil {
		t.Fatal(err)
	}
	if sec.Permission != "staff.manage" {
		t.Fatalf("permission decoded as %q", sec.Permission)
	}

	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{
			name: "view section loads",
			cfg: &Config{Views: map[string]ViewConfig{"v": {
				Entry:    ViewEntry{Type: "ticket"},
				Sections: []ViewSection{{Heading: "Staff", Source: "entry", Display: "content", SectionPresentation: SectionPresentation{SectionVisibility: SectionVisibility{Permission: "staff.manage"}}}},
			}}},
		},
		{
			name: "side panel section refused",
			cfg: &Config{Forms: map[string]Form{"ticket_form": {
				EntityType: "ticket",
				Fields:     []FormField{{Property: "title"}},
				SidePanel: &SidePanelConfig{
					Traverse: []ViewTraverse{{From: "entry", Follow: "belongs-to", CollectAs: "cats"}},
					Sections: []ViewSection{{Heading: "Cats", Source: "cats", Display: "cards", SectionPresentation: SectionPresentation{SectionVisibility: SectionVisibility{Permission: "staff.manage"}}}},
				},
			}}},
			want: "side_panel section[0] sets permission",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfig([]byte(`version: "1.0"`), tc.cfg, testMetamodel())
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("want clean load, got: %v", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Fatalf("want an error containing %q, got: %v", tc.want, err)
			}
		})
	}
}
