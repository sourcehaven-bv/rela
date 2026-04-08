package dataentryconfig

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestValidateHexColor(t *testing.T) {
	valid := []string{"#fff", "#FFF", "#aabbcc", "#AABBCC", "#aabbcc00", "#12345678"}
	for _, c := range valid {
		require.NoError(t, ValidateHexColor(c), "should accept %s", c)
	}

	invalid := []string{"red", "rgb(0,0,0)", "#gg0000", "#12", "#12345", "", "#000; url(evil)", "not-a-color"}
	for _, c := range invalid {
		require.Error(t, ValidateHexColor(c), "should reject %s", c)
	}
}

func TestValidatePalette(t *testing.T) {
	t.Run("nil palette is valid", func(t *testing.T) {
		assert.NoError(t, ValidatePalette(nil))
	})

	t.Run("valid partial palette", func(t *testing.T) {
		p := &PaletteConfig{PaletteColors: PaletteColors{Accent: "#ff0000"}}
		assert.NoError(t, ValidatePalette(p))
	})

	t.Run("invalid color value", func(t *testing.T) {
		p := &PaletteConfig{PaletteColors: PaletteColors{Accent: "not-hex"}}
		err := ValidatePalette(p)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "accent")
	})

	t.Run("unknown badge name", func(t *testing.T) {
		p := &PaletteConfig{Badges: map[string]string{"teal": "#00ffff"}}
		err := ValidatePalette(p)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown badge color")
	})

	t.Run("invalid badge color", func(t *testing.T) {
		p := &PaletteConfig{Badges: map[string]string{"blue": "invalid"}}
		err := ValidatePalette(p)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "badges.blue")
	})

	t.Run("valid badges", func(t *testing.T) {
		p := &PaletteConfig{Badges: map[string]string{"blue": "#1e40af", "red": "#dc2626"}}
		assert.NoError(t, ValidatePalette(p))
	})

	t.Run("invalid dark explicit color", func(t *testing.T) {
		p := &PaletteConfig{Dark: DarkMode{Explicit: &DarkPalette{PaletteColors: PaletteColors{Accent: "bad"}}}}
		err := ValidatePalette(p)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dark")
	})
}

func TestDarkModeUnmarshalYAML(t *testing.T) {
	t.Run("empty defaults to neither disabled nor explicit", func(t *testing.T) {
		var d DarkMode
		assert.False(t, d.IsDisabled())
		assert.False(t, d.IsExplicit())
	})

	t.Run("false bool", func(t *testing.T) {
		var d DarkMode
		require.NoError(t, yaml.Unmarshal([]byte(`false`), &d))
		assert.True(t, d.IsDisabled())
		assert.False(t, d.IsExplicit())
	})

	t.Run("true bool is rejected", func(t *testing.T) {
		var d DarkMode
		err := yaml.Unmarshal([]byte(`true`), &d)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid dark mode")
	})

	t.Run("auto string is rejected (legacy)", func(t *testing.T) {
		var d DarkMode
		err := yaml.Unmarshal([]byte(`auto`), &d)
		require.Error(t, err)
	})

	t.Run("disabled string is rejected", func(t *testing.T) {
		var d DarkMode
		err := yaml.Unmarshal([]byte(`disabled`), &d)
		require.Error(t, err)
	})

	t.Run("explicit palette", func(t *testing.T) {
		input := `
accent: "#818cf8"
surface: "#121218"
`
		var d DarkMode
		require.NoError(t, yaml.Unmarshal([]byte(input), &d))
		assert.True(t, d.IsExplicit())
		assert.Equal(t, "#818cf8", d.Explicit.Accent)
		assert.Equal(t, "#121218", d.Explicit.Surface)
	})

	t.Run("invalid string", func(t *testing.T) {
		var d DarkMode
		err := yaml.Unmarshal([]byte(`something_wrong`), &d)
		require.Error(t, err)
	})
}

func TestDarkModeMarshalYAML(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		d := DarkMode{Disabled: true}
		out, err := yaml.Marshal(d)
		require.NoError(t, err)
		assert.Contains(t, string(out), "false")
	})

	t.Run("explicit", func(t *testing.T) {
		d := DarkMode{Explicit: &DarkPalette{PaletteColors: PaletteColors{Accent: "#818cf8"}}}
		out, err := yaml.Marshal(d)
		require.NoError(t, err)
		assert.Contains(t, string(out), "#818cf8")
	})

	t.Run("zero value marshals to null", func(t *testing.T) {
		d := DarkMode{}
		out, err := yaml.Marshal(d)
		require.NoError(t, err)
		assert.Equal(t, "null\n", string(out))
	})
}

func TestDarkModeJSON(t *testing.T) {
	t.Run("marshal disabled", func(t *testing.T) {
		d := DarkMode{Disabled: true}
		out, err := json.Marshal(d)
		require.NoError(t, err)
		assert.Equal(t, "false", string(out))
	})

	t.Run("marshal explicit", func(t *testing.T) {
		d := DarkMode{Explicit: &DarkPalette{PaletteColors: PaletteColors{Accent: "#818cf8"}}}
		out, err := json.Marshal(d)
		require.NoError(t, err)
		assert.Contains(t, string(out), `"accent":"#818cf8"`)
	})

	t.Run("marshal zero value", func(t *testing.T) {
		d := DarkMode{}
		out, err := json.Marshal(d)
		require.NoError(t, err)
		assert.Equal(t, "null", string(out))
	})

	t.Run("unmarshal false bool", func(t *testing.T) {
		var d DarkMode
		require.NoError(t, json.Unmarshal([]byte(`false`), &d))
		assert.True(t, d.IsDisabled())
	})

	t.Run("unmarshal true bool is rejected", func(t *testing.T) {
		var d DarkMode
		err := json.Unmarshal([]byte(`true`), &d)
		require.Error(t, err)
	})

	t.Run("unmarshal auto string is rejected (legacy)", func(t *testing.T) {
		var d DarkMode
		err := json.Unmarshal([]byte(`"auto"`), &d)
		require.Error(t, err)
	})

	t.Run("unmarshal null", func(t *testing.T) {
		var d DarkMode
		require.NoError(t, json.Unmarshal([]byte(`null`), &d))
		assert.False(t, d.IsDisabled())
		assert.False(t, d.IsExplicit())
	})

	t.Run("unmarshal explicit object", func(t *testing.T) {
		var d DarkMode
		require.NoError(t, json.Unmarshal([]byte(`{"accent":"#818cf8","surface":"#121218"}`), &d))
		assert.True(t, d.IsExplicit())
		assert.Equal(t, "#818cf8", d.Explicit.Accent)
		assert.Equal(t, "#121218", d.Explicit.Surface)
	})

	t.Run("round-trip disabled", func(t *testing.T) {
		original := DarkMode{Disabled: true}
		data, err := json.Marshal(original)
		require.NoError(t, err)
		var decoded DarkMode
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.True(t, decoded.IsDisabled())
	})

	t.Run("round-trip explicit", func(t *testing.T) {
		original := DarkMode{Explicit: &DarkPalette{PaletteColors: PaletteColors{Accent: "#818cf8", Base: "#0f0f1a"}}}
		data, err := json.Marshal(original)
		require.NoError(t, err)
		var decoded DarkMode
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.True(t, decoded.IsExplicit())
		assert.Equal(t, original.Explicit.Accent, decoded.Explicit.Accent)
		assert.Equal(t, original.Explicit.Base, decoded.Explicit.Base)
	})

	t.Run("full PaletteConfig JSON round-trip", func(t *testing.T) {
		original := PaletteConfig{
			PaletteColors: PaletteColors{Accent: "#e11d48"},
			Badges:        map[string]string{"blue": "#1e40af"},
			Dark:          DarkMode{Explicit: &DarkPalette{PaletteColors: PaletteColors{Accent: "#818cf8"}}},
		}
		data, err := json.Marshal(original)
		require.NoError(t, err)
		var decoded PaletteConfig
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.Equal(t, "#e11d48", decoded.Accent)
		assert.Equal(t, "#1e40af", decoded.Badges["blue"])
		assert.True(t, decoded.Dark.IsExplicit())
		assert.Equal(t, "#818cf8", decoded.Dark.Explicit.Accent)
	})
}

func TestResolvePalette(t *testing.T) {
	t.Run("nil palettes return defaults with dark disabled", func(t *testing.T) {
		r := ResolvePalette(nil, nil)
		require.NotNil(t, r)
		assert.Equal(t, defaultLightColors.Accent, r.Light["--accent-color"])
		assert.Equal(t, defaultBadgeColors["blue"], r.Light["--badge-blue"])
		assert.True(t, r.DarkDisabled, "dark should be disabled when no explicit dark configured")
		assert.Empty(t, r.Dark)
	})

	t.Run("partial project palette merges with defaults", func(t *testing.T) {
		project := &PaletteConfig{PaletteColors: PaletteColors{Accent: "#e11d48"}}
		r := ResolvePalette(project, nil)
		assert.Equal(t, "#e11d48", r.Light["--accent-color"])
		assert.Equal(t, defaultLightColors.Surface, r.Light["--bg-color"]) // default
	})

	t.Run("user palette overrides project", func(t *testing.T) {
		project := &PaletteConfig{PaletteColors: PaletteColors{Accent: "#e11d48"}}
		user := &PaletteConfig{PaletteColors: PaletteColors{Accent: "#0ea5e9"}}
		r := ResolvePalette(project, user)
		assert.Equal(t, "#0ea5e9", r.Light["--accent-color"])
	})

	t.Run("badge overrides", func(t *testing.T) {
		project := &PaletteConfig{Badges: map[string]string{"blue": "#1e40af"}}
		r := ResolvePalette(project, nil)
		assert.Equal(t, "#1e40af", r.Light["--badge-blue"])
		assert.Equal(t, defaultBadgeColors["red"], r.Light["--badge-red"]) // unchanged
	})

	t.Run("dark explicitly disabled", func(t *testing.T) {
		project := &PaletteConfig{Dark: DarkMode{Disabled: true}}
		r := ResolvePalette(project, nil)
		assert.True(t, r.DarkDisabled)
		assert.Empty(t, r.Dark)
	})

	t.Run("fully-explicit dark palette", func(t *testing.T) {
		project := &PaletteConfig{
			Dark: DarkMode{Explicit: &DarkPalette{PaletteColors: PaletteColors{
				Base: "#0a0a14", Surface: "#121218", Accent: "#818cf8", Text: "#e8e8f0",
				Success: "#34d399", Error: "#f87171", Warning: "#fbbf24", Info: "#60a5fa",
			}}},
		}
		r := ResolvePalette(project, nil)
		assert.False(t, r.DarkDisabled)
		assert.Equal(t, "#818cf8", r.Dark["--accent-color"])
		assert.Equal(t, "#121218", r.Dark["--bg-color"])
		assert.Equal(t, "#0a0a14", r.Dark["--sidebar-bg"])
	})

	t.Run("partial explicit dark palette inherits unset fields from light", func(t *testing.T) {
		// User specified only the dark accent. The other dark slots
		// should fall back to the resolved light palette so nothing
		// from defaultLightColors leaks into the rendered theme.
		project := &PaletteConfig{
			PaletteColors: PaletteColors{Surface: "#fafafa"},
			Dark:          DarkMode{Explicit: &DarkPalette{PaletteColors: PaletteColors{Accent: "#818cf8"}}},
		}
		r := ResolvePalette(project, nil)
		assert.Equal(t, "#818cf8", r.Dark["--accent-color"])
		assert.Equal(t, "#fafafa", r.Dark["--bg-color"], "unset dark surface should inherit from light")
	})

	t.Run("explicit dark badges override light badges", func(t *testing.T) {
		project := &PaletteConfig{
			Badges: map[string]string{"blue": "#1e40af", "red": "#dc2626"},
			Dark: DarkMode{Explicit: &DarkPalette{
				PaletteColors: PaletteColors{Accent: "#818cf8"},
				Badges:        map[string]string{"blue": "#5fa9ff"},
			}},
		}
		r := ResolvePalette(project, nil)
		// dark.blue is explicitly overridden
		assert.Equal(t, "#5fa9ff", r.Dark["--badge-blue"])
		// dark.red was not set → inherits from the resolved light badges
		assert.Equal(t, "#dc2626", r.Dark["--badge-red"])
	})

	t.Run("user dark overrides project dark", func(t *testing.T) {
		project := &PaletteConfig{Dark: DarkMode{Disabled: true}}
		user := &PaletteConfig{Dark: DarkMode{Explicit: &DarkPalette{PaletteColors: PaletteColors{Accent: "#abcdef"}}}}
		r := ResolvePalette(project, user)
		assert.False(t, r.DarkDisabled)
		assert.Equal(t, "#abcdef", r.Dark["--accent-color"])
	})

	t.Run("derived light variables present", func(t *testing.T) {
		r := ResolvePalette(nil, nil)
		derivedKeys := []string{"--card-bg", "--input-bg", "--hover-bg", "--border-color", "--muted-text", "--sidebar-text"}
		for _, key := range derivedKeys {
			assert.NotEmpty(t, r.Light[key], "light missing %s", key)
		}
	})

	t.Run("21 light variables when dark disabled", func(t *testing.T) {
		r := ResolvePalette(nil, nil)
		assert.Len(t, r.Light, 21) // 8 base + 6 derived + 7 badges
		assert.Empty(t, r.Dark)
	})

	t.Run("zero-value DarkMode (from JSON null) is treated as disabled", func(t *testing.T) {
		// JSON `null` decoded into DarkMode produces a zero value:
		// neither Disabled nor Explicit set. Without the defensive
		// check this would panic on `*darkMode.Explicit`.
		var p PaletteConfig
		require.NoError(t, json.Unmarshal([]byte(`{"accent":"#ffcd75","dark":null}`), &p))
		assert.False(t, p.Dark.IsDisabled())
		assert.False(t, p.Dark.IsExplicit())

		require.NotPanics(t, func() {
			r := ResolvePalette(nil, &p)
			assert.True(t, r.DarkDisabled)
		})
	})
}

func TestHSLConversion(t *testing.T) {
	t.Run("black", func(t *testing.T) {
		h := hexToHSL("#000000")
		assert.InDelta(t, 0, h.L, 0.01)
	})

	t.Run("white", func(t *testing.T) {
		h := hexToHSL("#ffffff")
		assert.InDelta(t, 1.0, h.L, 0.01)
	})

	t.Run("round trip", func(t *testing.T) {
		colors := []string{"#3b82f6", "#ef4444", "#10b981", "#1a1a2e", "#f8fafc"}
		for _, c := range colors {
			h := hexToHSL(c)
			result := hslToHex(h)
			assert.Equal(t, c, result, "round trip failed for %s", c)
		}
	})

	t.Run("3-digit hex normalized", func(t *testing.T) {
		assert.Equal(t, "#ff0000", normalizeHex("#f00"))
	})

	t.Run("8-digit hex strips alpha", func(t *testing.T) {
		assert.Equal(t, "#ff0000", normalizeHex("#ff000080"))
	})
}

func TestDeriveEdgeCases(t *testing.T) {
	t.Run("white surface clamps card-bg", func(t *testing.T) {
		colors := defaultLightColors
		colors.Surface = "#ffffff"
		badges := copyBadges(defaultBadgeColors)
		theme := deriveTheme(colors, badges)
		assert.Equal(t, "#ffffff", theme["--card-bg"])
		assert.Equal(t, "#ffffff", theme["--input-bg"])
	})

	t.Run("black surface clamps hover-bg", func(t *testing.T) {
		colors := defaultLightColors
		colors.Surface = "#000000"
		badges := copyBadges(defaultBadgeColors)
		theme := deriveTheme(colors, badges)
		assert.Equal(t, "#000000", theme["--hover-bg"])
	})

	t.Run("dark base gets light sidebar-text", func(t *testing.T) {
		colors := defaultLightColors
		colors.Base = "#000000"
		badges := copyBadges(defaultBadgeColors)
		theme := deriveTheme(colors, badges)
		assert.Equal(t, "#e8e8e8", theme["--sidebar-text"])
	})

	t.Run("light base gets dark sidebar-text", func(t *testing.T) {
		colors := defaultLightColors
		colors.Base = "#ffffff"
		badges := copyBadges(defaultBadgeColors)
		theme := deriveTheme(colors, badges)
		assert.Equal(t, "#1e293b", theme["--sidebar-text"])
	})
}

func TestPaletteConfigYAML(t *testing.T) {
	input := `
palette:
  accent: "#e11d48"
  surface: "#fafafa"
  badges:
    blue: "#1e40af"
  dark: false
`
	var cfg struct {
		Palette *PaletteConfig `yaml:"palette"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(input), &cfg))
	require.NotNil(t, cfg.Palette)
	assert.Equal(t, "#e11d48", cfg.Palette.Accent)
	assert.Equal(t, "#fafafa", cfg.Palette.Surface)
	assert.Equal(t, "#1e40af", cfg.Palette.Badges["blue"])
	assert.True(t, cfg.Palette.Dark.IsDisabled())
}
