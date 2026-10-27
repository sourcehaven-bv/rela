package dataentryconfig

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseColor(t *testing.T) {
	operator := map[string]OperatorColor{"risico-kritiek": {Light: "#7a0010"}}
	tests := []struct {
		name  string
		class string
	}{
		{"red", "badge-red"},
		{"gray", "badge-gray"},
		{"grey", "badge-gray"},
		{"purple", "badge-purple"},
		{"yellow", "badge-yellow"},
		{"teal", "badge-teal"},
		{"red/700", "kit-red-700"},
		{"gray/50", "kit-grey-50"},
		{"teal/900", "kit-teal-900"},
		{"risico-kritiek", "custom-risico-kritiek"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := ParseColor(tc.name, operator)
			require.NoError(t, err)
			assert.Equal(t, tc.class, c.StyleClass())
		})
	}

	for _, bad := range []string{"pink", "cyan", "Red", "red/750", "red/050", "red/", "/700", "purple/500", "", "risico"} {
		t.Run("rejects "+bad, func(t *testing.T) {
			_, err := ParseColor(bad, operator)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unknown color")
		})
	}
}

// TestForegroundForReachesAA pins the automatic text color: AA on every kit
// shade and on random colors, which is the guarantee the docs make.
func TestForegroundForReachesAA(t *testing.T) {
	for hue, ramp := range kitRamps {
		for i, hex := range ramp {
			fg := ForegroundFor(hex)
			assert.GreaterOrEqual(t, contrastRatio(hex, fg), aaText, "%s/%s %s on %s", hue, kitShades[i], fg, hex)
		}
	}
	r := rand.New(rand.NewPCG(1, 2))
	for range 5000 {
		hex := fmt.Sprintf("#%06x", r.IntN(0x1000000))
		assert.GreaterOrEqual(t, contrastRatio(hex, ForegroundFor(hex)), aaText, hex)
	}
	assert.Equal(t, "#ffffff", ForegroundFor("#000000"))
	assert.Equal(t, darkText, ForegroundFor("#ffffff"))
}

// TestKitRampsMatchTokens pins the Go copy of the kit to the palette the SPA
// paints with, light and mirrored dark, and each shade's text color to the
// one ForegroundFor chooses.
func TestKitRampsMatchTokens(t *testing.T) {
	css, err := os.ReadFile("../dataentry/apps_tokens.css")
	require.NoError(t, err)
	light, media, ok := strings.Cut(string(css), "@media (prefers-color-scheme: dark)")
	require.True(t, ok, "apps_tokens.css has no prefers-color-scheme block")
	media, darkBlock, ok := strings.Cut(media, "\n.dark {")
	require.True(t, ok, "apps_tokens.css has no .dark block")

	tokens := func(block string) map[string]string {
		m := map[string]string{}
		re := regexp.MustCompile(`(--rl-kit-[a-z]+-\d+(?:-fg)?):\s*(#[0-9a-f]{6});`)
		for _, match := range re.FindAllStringSubmatch(block, -1) {
			m[match[1]] = match[2]
		}
		return m
	}
	lt, dt := tokens(light), tokens(darkBlock)
	require.Len(t, lt, 2*len(kitRamps)*len(kitShades))
	assert.Equal(t, dt, tokens(media), "both dark blocks carry the same kit tokens")

	for hue, ramp := range kitRamps {
		for i, shade := range kitShades {
			tok := "--rl-kit-" + hue + "-" + shade
			assert.Equal(t, ramp[i], lt[tok], tok)
			assert.Equal(t, ForegroundFor(ramp[i]), lt[tok+"-fg"], tok+"-fg")
			mirror := ramp[len(ramp)-1-i]
			assert.Equal(t, mirror, dt[tok], "dark "+tok)
			assert.Equal(t, ForegroundFor(mirror), dt[tok+"-fg"], "dark "+tok+"-fg")
		}
	}
}

func TestValidatePalette_Colors(t *testing.T) {
	valid := &PaletteConfig{
		Colors: map[string]OperatorColor{
			"risico-kritiek": {Light: "#7a0010", Dark: "#ff6b6b"},
			"brand":          {Light: "#abc"},
			"red-700":        {Light: "#123456"},
		},
		Dark: DarkMode{Explicit: &DarkPalette{Colors: map[string]string{"red/700": "#aa0000", "grey/50": "#111"}}},
	}
	require.NoError(t, ValidatePalette(valid))

	tests := []struct {
		name string
		p    *PaletteConfig
		want string
	}{
		{"kit name", &PaletteConfig{Colors: map[string]OperatorColor{"red": {Light: "#000"}}}, "reserved"},
		{"alias", &PaletteConfig{Colors: map[string]OperatorColor{"gray": {Light: "#000"}}}, "reserved"},
		{"tone", &PaletteConfig{Colors: map[string]OperatorColor{"warning": {Light: "#000"}}}, "reserved"},
		{"slash", &PaletteConfig{Colors: map[string]OperatorColor{"red/950": {Light: "#000"}}}, "lower-case"},
		{"upper", &PaletteConfig{Colors: map[string]OperatorColor{"Brand": {Light: "#000"}}}, "lower-case"},
		{"css injection", &PaletteConfig{Colors: map[string]OperatorColor{"a;b": {Light: "#000"}}}, "lower-case"},
		{"token clash", &PaletteConfig{Colors: map[string]OperatorColor{"brand-fg": {Light: "#000"}}}, "must not end in -fg"},
		{"gray and grey", &PaletteConfig{Badges: map[string]string{"gray": "#111", "grey": "#222"}}, "not both"},
		{
			"dark gray and grey",
			&PaletteConfig{Dark: DarkMode{Explicit: &DarkPalette{Badges: map[string]string{"gray": "#111", "grey": "#222"}}}},
			"not both",
		},
		{"no light", &PaletteConfig{Colors: map[string]OperatorColor{"x": {Dark: "#000"}}}, "light is required"},
		{"bad hex", &PaletteConfig{Colors: map[string]OperatorColor{"x": {Light: "red"}}}, "invalid color"},
		{"alpha", &PaletteConfig{Colors: map[string]OperatorColor{"x": {Light: "#00000080"}}}, "invalid color"},
		{"bad dark", &PaletteConfig{Colors: map[string]OperatorColor{"x": {Light: "#000", Dark: "#12"}}}, "invalid color"},
		{
			"dark override of a bare hue",
			&PaletteConfig{Dark: DarkMode{Explicit: &DarkPalette{Colors: map[string]string{"red": "#000"}}}},
			"not a kit shade",
		},
		{
			"dark override bad hex",
			&PaletteConfig{Dark: DarkMode{Explicit: &DarkPalette{Colors: map[string]string{"red/700": "#zzz"}}}},
			"invalid color",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePalette(tc.p)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestResolvePalette_OperatorColors(t *testing.T) {
	project := &PaletteConfig{Colors: map[string]OperatorColor{
		"kritiek": {Light: "#7a0010", Dark: "#ff6b6b"},
		"brand":   {Light: "#abc"},
	}}
	r := ResolvePalette(project, nil)

	assert.Equal(t, "#7a0010", r.Light["--rl-custom-kritiek"])
	assert.Equal(t, "#ffffff", r.Light["--rl-custom-kritiek-fg"])
	assert.Equal(t, "#ff6b6b", r.Dark["--rl-custom-kritiek"])
	assert.Equal(t, ForegroundFor("#ff6b6b"), r.Dark["--rl-custom-kritiek-fg"])
	assert.Equal(t, "#aabbcc", r.Light["--rl-custom-brand"], "short hex is expanded")
	assert.Equal(t, "#aabbcc", r.Dark["--rl-custom-brand"], "dark falls back to light")

	user := &PaletteConfig{Colors: map[string]OperatorColor{"brand": {Light: "#000000"}}}
	assert.Equal(t, "#000000", ResolvePalette(project, user).Light["--rl-custom-brand"], "user wins per name")

	disabled := &PaletteConfig{Colors: project.Colors, Dark: DarkMode{Disabled: true}}
	assert.Equal(t, "#7a0010", ResolvePalette(disabled, nil).Light["--rl-custom-kritiek"])
}

func TestResolvePalette_DarkKitOverride(t *testing.T) {
	project := &PaletteConfig{Dark: DarkMode{Explicit: &DarkPalette{Colors: map[string]string{"red/700": "#aa0000"}}}}
	r := ResolvePalette(project, nil)

	assert.Equal(t, "#aa0000", r.Dark["--rl-kit-red-700"])
	assert.Equal(t, ForegroundFor("#aa0000"), r.Dark["--rl-kit-red-700-fg"])
	// The light map restates the kit value, so leaving dark mode does not
	// keep the dark override inline.
	assert.Equal(t, "#dc252b", r.Light["--rl-kit-red-700"])
	assert.Equal(t, ForegroundFor("#dc252b"), r.Light["--rl-kit-red-700-fg"])

	for name := range r.Light {
		if strings.HasPrefix(name, "--rl-kit-") {
			assert.Contains(t, r.Dark, name, "every kit key set in light is set in dark too")
		}
	}
	none := ResolvePalette(nil, nil)
	assert.False(t, slices.ContainsFunc(keys(none.Light), func(k string) bool { return strings.HasPrefix(k, "--rl-kit-") }),
		"no override emits no kit tokens; tokens.css carries them")
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestValidateConfig_StyleColors(t *testing.T) {
	ok := `
version: "1.0"
palette:
  colors:
    risico-kritiek: {light: '#7a0010'}
styles:
  priority: {high: red/900, medium: yellow, low: teal/300}
  status: {open: grey, in-progress: risico-kritiek, closed: gray}
`
	_, err := loadAndValidate(t, ok)
	require.NoError(t, err)

	bad := `
version: "1.0"
palette:
  accent: nothex
styles:
  priority: {high: pink, medium: red/750, low: purple/500}
`
	_, err = loadAndValidate(t, bad)
	require.Error(t, err)
	for _, want := range []string{
		`styles.priority.high: unknown color "pink"`,
		`styles.priority.medium: unknown color "red/750"`,
		`styles.priority.low: unknown color "purple/500"`,
		"palette: accent",
	} {
		assert.Contains(t, err.Error(), want)
	}
}

// TestRLDefaultTokensMatchTokensCSS pins the restated defaults to the palette
// the SPA ships, so restating a key never paints a stale value.
func TestRLDefaultTokensMatchTokensCSS(t *testing.T) {
	css, err := os.ReadFile("../dataentry/apps_tokens.css")
	require.NoError(t, err)
	light, rest, ok := strings.Cut(string(css), "@media (prefers-color-scheme: dark)")
	require.True(t, ok)
	_, dark, ok := strings.Cut(rest, "\n.dark {")
	require.True(t, ok)
	value := func(block, name string) string {
		m := regexp.MustCompile(regexp.QuoteMeta(name) + `:\s*(#[0-9a-f]{6});`).FindStringSubmatch(block)
		if m == nil {
			return ""
		}
		return m[1]
	}
	require.Equal(t, slices.Sorted(maps.Keys(rlDefaultDarkTokens)), slices.Sorted(maps.Keys(rlDefaultLightTokens)))
	for name, want := range rlDefaultLightTokens {
		assert.Equal(t, want, value(light, name), name)
	}
	for name, want := range rlDefaultDarkTokens {
		assert.Equal(t, want, value(dark, name), "dark "+name)
	}
}

// TestResolvePalette_ThemesSetTheSameRLKeys: the SPA never removes an inline
// key, so a key set in one theme only would survive a switch to the other.
func TestResolvePalette_ThemesSetTheSameRLKeys(t *testing.T) {
	rl := func(m map[string]string) []string {
		var out []string
		for k := range m {
			if strings.HasPrefix(k, "--rl-") {
				out = append(out, k)
			}
		}
		slices.Sort(out)
		return out
	}
	darkOnly := &PaletteConfig{Dark: DarkMode{Explicit: &DarkPalette{
		PaletteColors: PaletteColors{Accent: "#abcdef"}, Badges: map[string]string{"teal": "#4db6ac", "grey": "#999999"},
	}}}
	cases := map[string]*PaletteConfig{
		"light only": {PaletteColors: PaletteColors{Accent: "#123456"}, Badges: map[string]string{"teal": "#00695c"}},
		"dark only":  darkOnly,
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			r := ResolvePalette(p, nil)
			assert.Equal(t, rl(r.Light), rl(r.Dark))
		})
	}
	r := ResolvePalette(darkOnly, nil)
	assert.Equal(t, "#999999", r.Dark["--rl-color-status-grey"], "dark grey alias reaches the tokens")
	assert.Equal(t, rlDefaultLightTokens["--rl-color-status-grey"], r.Light["--rl-color-status-grey"])
}
