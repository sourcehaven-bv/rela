package dataentryconfig

import (
	"strings"
	"testing"
)

func TestSectionHideIdentity_Accepts(t *testing.T) {
	for _, display := range []string{"list", "cards", "content"} {
		t.Run(display, func(t *testing.T) {
			cfg := sectionCreateConfig(nil)
			sec := &cfg.Views["ticket_detail"].Sections[0]
			sec.Display = display
			sec.HideType, sec.HideID = true, true
			if got := validateOneView(t, cfg); got != "" {
				t.Fatalf("hide_type and hide_id must load on %s, got: %s", display, got)
			}
		})
	}
}

func TestSectionHideIdentity_Refuses(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ViewSection)
	}{
		{"on the entry", func(s *ViewSection) { s.Source = "entry"; s.Display = "properties" }},
		{"on a table", func(s *ViewSection) { s.Display = "table" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := sectionCreateConfig(nil)
			sec := &cfg.Views["ticket_detail"].Sections[0]
			sec.HideID = true
			tt.mutate(sec)
			if got := validateOneView(t, cfg); !strings.Contains(got, "hide_type or hide_id") {
				t.Errorf("want a hide_type or hide_id error, got: %s", got)
			}
		})
	}
}

func TestValidateSectionHideIdentity_Nested(t *testing.T) {
	ok := ViewSection{Source: "x", Display: DisplayNested, HideID: true}
	if errs := validateSectionHideIdentity("v", 0, ok); len(errs) != 0 {
		t.Errorf("hide_id on nested must pass, got %v", errs)
	}
	bad := ViewSection{Source: "x", Display: DisplayNested, HideType: true}
	if errs := validateSectionHideIdentity("v", 0, bad); len(errs) != 1 {
		t.Errorf("hide_type on nested must be refused, got %v", errs)
	}
}

func TestValidateConfig_MentionedByHideIDOnlyEntry(t *testing.T) {
	cfg := &Config{EntityViews: map[string]EntityViewConfig{"ticket": {MentionedByHideID: true}}}
	if err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel()); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}
