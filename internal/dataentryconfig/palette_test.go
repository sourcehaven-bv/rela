package dataentryconfig

import (
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
		p := &PaletteConfig{Dark: DarkMode{Explicit: &PaletteColors{Accent: "bad"}}}
		err := ValidatePalette(p)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dark")
	})
}

func TestDarkModeUnmarshalYAML(t *testing.T) {
	t.Run("auto string", func(t *testing.T) {
		var d DarkMode
		require.NoError(t, yaml.Unmarshal([]byte(`auto`), &d))
		assert.True(t, d.IsAuto())
		assert.False(t, d.IsDisabled())
		assert.False(t, d.IsExplicit())
	})

	t.Run("empty defaults to auto", func(t *testing.T) {
		var d DarkMode
		// When omitted, zero value
		assert.True(t, d.IsAuto())
	})

	t.Run("false bool", func(t *testing.T) {
		var d DarkMode
		require.NoError(t, yaml.Unmarshal([]byte(`false`), &d))
		assert.True(t, d.IsDisabled())
		assert.False(t, d.IsAuto())
	})

	t.Run("disabled string", func(t *testing.T) {
		var d DarkMode
		require.NoError(t, yaml.Unmarshal([]byte(`disabled`), &d))
		assert.True(t, d.IsDisabled())
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
		assert.Contains(t, err.Error(), "invalid dark mode")
	})
}

func TestDarkModeMarshalYAML(t *testing.T) {
	t.Run("auto", func(t *testing.T) {
		d := DarkMode{Mode: "auto"}
		out, err := yaml.Marshal(d)
		require.NoError(t, err)
		assert.Contains(t, string(out), "auto")
	})

	t.Run("false", func(t *testing.T) {
		d := DarkMode{Mode: "false"}
		out, err := yaml.Marshal(d)
		require.NoError(t, err)
		assert.Contains(t, string(out), "false")
	})

	t.Run("explicit", func(t *testing.T) {
		d := DarkMode{Explicit: &PaletteColors{Accent: "#818cf8"}}
		out, err := yaml.Marshal(d)
		require.NoError(t, err)
		assert.Contains(t, string(out), "#818cf8")
	})
}

func TestResolvePalette(t *testing.T) {
	t.Run("nil palettes return defaults", func(t *testing.T) {
		r := ResolvePalette(nil, nil)
		require.NotNil(t, r)
		assert.Equal(t, defaultLightColors.Accent, r.Light["--accent-color"])
		assert.Equal(t, defaultBadgeColors["blue"], r.Light["--badge-blue"])
		assert.NotEmpty(t, r.Dark) // auto dark generated
		assert.False(t, r.DarkDisabled)
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

	t.Run("dark disabled", func(t *testing.T) {
		project := &PaletteConfig{Dark: DarkMode{Mode: "false"}}
		r := ResolvePalette(project, nil)
		assert.True(t, r.DarkDisabled)
		assert.Empty(t, r.Dark)
	})

	t.Run("explicit dark palette", func(t *testing.T) {
		project := &PaletteConfig{
			Dark: DarkMode{Explicit: &PaletteColors{Accent: "#818cf8", Surface: "#121218"}},
		}
		r := ResolvePalette(project, nil)
		assert.Equal(t, "#818cf8", r.Dark["--accent-color"])
		assert.Equal(t, "#121218", r.Dark["--bg-color"])
		assert.False(t, r.DarkDisabled)
	})

	t.Run("auto dark is generated", func(t *testing.T) {
		r := ResolvePalette(nil, nil)
		// Dark surface should be much darker than light surface
		assert.NotEqual(t, r.Light["--bg-color"], r.Dark["--bg-color"])
		// Dark text should be lighter than light text
		assert.NotEqual(t, r.Light["--text-color"], r.Dark["--text-color"])
	})

	t.Run("derived variables present", func(t *testing.T) {
		r := ResolvePalette(nil, nil)
		derivedKeys := []string{"--card-bg", "--input-bg", "--hover-bg", "--border-color", "--muted-text", "--sidebar-text"}
		for _, key := range derivedKeys {
			assert.NotEmpty(t, r.Light[key], "light missing %s", key)
			assert.NotEmpty(t, r.Dark[key], "dark missing %s", key)
		}
	})

	t.Run("all 21 variables in light and dark", func(t *testing.T) {
		r := ResolvePalette(nil, nil)
		assert.Len(t, r.Light, 21) // 8 base + 6 derived + 7 badges
		assert.Len(t, r.Dark, 21)
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
