package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func validManifest() *ThemeManifest {
	return &ThemeManifest{
		Name:    "Acme Theme",
		Version: "1.0.0",
		Author:  "Acme Inc.",
		Logo:    "logo.png",
		PaletteConfig: PaletteConfig{
			PaletteColors: PaletteColors{
				Accent: "#6366f1",
			},
		},
	}
}

func TestValidateThemeManifest_Accepts(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*ThemeManifest)
	}{
		{"all fields", func(*ThemeManifest) {}},
		{"no author", func(m *ThemeManifest) { m.Author = "" }},
		{"no logo", func(m *ThemeManifest) { m.Logo = "" }},
		{"no palette", func(m *ThemeManifest) { m.PaletteConfig = PaletteConfig{} }},
		{"name at lower bound", func(m *ThemeManifest) { m.Name = "x" }},
		{"name at upper bound", func(m *ThemeManifest) { m.Name = strings.Repeat("a", maxThemeNameLen) }},
		{"version at upper bound", func(m *ThemeManifest) { m.Version = strings.Repeat("v", maxThemeVersionLen) }},
		{"author at upper bound", func(m *ThemeManifest) { m.Author = strings.Repeat("a", maxThemeAuthorLen) }},
		{"logo svg", func(m *ThemeManifest) { m.Logo = "logo.svg" }},
		{"logo jpg alias", func(m *ThemeManifest) { m.Logo = "logo.jpg" }},
		{"logo webp", func(m *ThemeManifest) { m.Logo = "logo.webp" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validManifest()
			tt.mut(m)
			if err := ValidateThemeManifest(m); err != nil {
				t.Errorf("expected accept, got %v", err)
			}
		})
	}
}

func TestValidateThemeManifest_Rejects(t *testing.T) {
	tests := []struct {
		name      string
		mut       func(*ThemeManifest)
		errSubstr string
	}{
		{"nil manifest", nil, "nil"},
		{"empty name", func(m *ThemeManifest) { m.Name = "" }, "name"},
		{"name too long", func(m *ThemeManifest) { m.Name = strings.Repeat("a", maxThemeNameLen+1) }, "name"},
		{"empty version", func(m *ThemeManifest) { m.Version = "" }, "version"},
		{"version too long", func(m *ThemeManifest) { m.Version = strings.Repeat("v", maxThemeVersionLen+1) }, "version"},
		{"author too long", func(m *ThemeManifest) { m.Author = strings.Repeat("a", maxThemeAuthorLen+1) }, "author"},
		{"logo with slash", func(m *ThemeManifest) { m.Logo = "sub/logo.png" }, "logo"},
		{"logo with backslash", func(m *ThemeManifest) { m.Logo = `sub\logo.png` }, "logo"},
		{"logo with parent ref", func(m *ThemeManifest) { m.Logo = "..logo.png" }, "logo"},
		{"logo without prefix", func(m *ThemeManifest) { m.Logo = "image.png" }, "logo"},
		{"logo bare prefix", func(m *ThemeManifest) { m.Logo = "logo." }, "logo"},
		{"logo no extension", func(m *ThemeManifest) { m.Logo = "logo" }, "logo"},
		{"logo unsupported extension", func(m *ThemeManifest) { m.Logo = "logo.tar.gz" }, "unsupported extension"},
		{"logo gif extension", func(m *ThemeManifest) { m.Logo = "logo.gif" }, "unsupported extension"},
		{"bad palette hex", func(m *ThemeManifest) { m.Accent = "not-a-color" }, "palette"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m *ThemeManifest
			if tt.mut != nil {
				m = validManifest()
				tt.mut(m)
			}
			err := ValidateThemeManifest(m)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("expected error containing %q, got %q", tt.errSubstr, err.Error())
			}
		})
	}
}

// TestCheckManifestTagsUnique guarantees the package-init guard works
// against the actual ThemeManifest declaration. If a future palette
// field introduces a yaml-tag shadow, yaml.v3 would panic on import;
// init catches it at startup, this test pins the contract.
func TestCheckManifestTagsUnique(t *testing.T) {
	if err := checkManifestTagsUnique(); err != nil {
		t.Errorf("expected no tag collisions in ThemeManifest, got: %v", err)
	}
}

// TestThemeManifest_YAMLRoundTrip pins down the on-disk shape so a
// future palette-field collision (e.g. someone adds a `name:` field to
// PaletteConfig) is caught loudly. Top-level keys are spelled here.
func TestThemeManifest_YAMLRoundTrip(t *testing.T) {
	src := `name: My Theme
version: 1.0.0
author: Me
logo: logo.png
accent: "#6366f1"
text: "#222222"
badges:
  blue: "#1e40af"
`
	var m ThemeManifest
	if err := yaml.Unmarshal([]byte(src), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Name != "My Theme" || m.Version != "1.0.0" || m.Author != "Me" || m.Logo != "logo.png" {
		t.Errorf("metadata fields not parsed correctly: %+v", m)
	}
	if m.Accent != "#6366f1" || m.Text != "#222222" {
		t.Errorf("palette colors not inlined: %+v", m.PaletteColors)
	}
	if m.Badges["blue"] != "#1e40af" {
		t.Errorf("badges not parsed: %+v", m.Badges)
	}

	// Re-marshal and verify the top-level keys appear once.
	out, err := yaml.Marshal(&m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	str := string(out)
	for _, key := range []string{"name:", "version:", "author:", "logo:", "accent:"} {
		if strings.Count(str, key) != 1 {
			t.Errorf("expected exactly one occurrence of %q in marshaled output:\n%s", key, str)
		}
	}
}
