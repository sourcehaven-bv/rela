package dataentryconfig

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
)

// The color vocabulary of `styles:` and the dashboard matrix (TKT-ZKNWSK).
//
// A color name is one of three forms:
//
//   - a bare hue (`red`), which keeps the tag look a style color always had;
//   - a kit shade (`red/700`), a filled color from the Atlas UI kit;
//   - an operator color, a name the project defines under `palette.colors`.
//
// The SPA paints a kit shade from the `--rl-kit-<hue>-<shade>` tokens in the
// rela-components palette, which also holds the dark mirror and a text color
// for each shade. kitRamps is the Go copy of the light values; the server needs
// it to restate a shade in light mode when `palette.dark.colors` overrides it.
// TestKitRampsMatchTokens pins it to apps_tokens.css.

// kitShades are the shade steps of every kit ramp, darkest first. The dark
// theme mirrors them: shade i shows the value of shade len-1-i.
var kitShades = []string{"900", "800", "700", "600", "500", "400", "300", "200", "100", "50"}

// kitRamps holds the light value of every kit shade, darkest first, in the
// order of kitShades.
var kitRamps = map[string][]string{
	"grey": {
		"#25272d", "#4a4d59", "#74798b", "#8f929c", "#d3d6dd",
		"#e2e4e9", "#ecedf0", "#f1f2f4", "#f3f4f6", "#f8f8fa",
	},
	"red": {
		"#c10616", "#cf1d24", "#dc252b", "#ee3131", "#fc3b31",
		"#f74f4d", "#ed7172", "#f59999", "#ffcdd2", "#ffebee",
	},
	"orange": {
		"#e65200", "#ef6d00", "#f57d00", "#fb8d00", "#ff9800",
		"#ffa826", "#ffb84d", "#ffcd80", "#ffe0b2", "#fff3e0",
	},
	"yellow": {
		"#f57f17", "#f9a825", "#fbc02d", "#fdd835", "#ffeb3b",
		"#ffee58", "#fff176", "#fff59d", "#fff9c4", "#fffde7",
	},
	"green": {
		"#007a34", "#008539", "#009942", "#00ad4e", "#00bc58",
		"#28c772", "#5fd28c", "#92dead", "#beebcd", "#e4f7eb",
	},
	"teal": {
		"#004d40", "#00695c", "#00796b", "#008f80", "#009688",
		"#26a69a", "#4db6ac", "#80cbc4", "#b2dfdb", "#e0f2f1",
	},
	"blue": {
		"#4656b9", "#5564bf", "#5b75d2", "#6287e6", "#6696f5",
		"#70a5f8", "#82b5fa", "#a2cafd", "#c5defe", "#e6f2fe",
	},
}

// bareHues are the hues valid without a shade. `purple` has no kit ramp, so it
// is bare only. `gray` is an alias of `grey`, the spelling style colors used
// before the kit.
var bareHues = map[string]bool{
	"grey": true, "red": true, "orange": true, "yellow": true,
	"green": true, "teal": true, "blue": true, "purple": true,
}

// hueAliases maps an accepted spelling to its hue.
var hueAliases = map[string]string{"gray": "grey"}

// toneHues are the tone names a dashboard matrix accepts besides color names,
// each with the kit hue it paints. An operator color may not take one of these
// names, so a matrix `colors:` value is never ambiguous.
var toneHues = map[string]string{
	"success": "green",
	"warning": "orange",
	"error":   "red",
	"info":    "blue",
	"new":     "purple",
}

// SemanticTones is the set of tone names, derived from toneHues so the names
// validation accepts and the names ToneColor paints cannot drift apart.
var SemanticTones = func() map[string]bool {
	out := make(map[string]bool, len(toneHues))
	for tone := range toneHues {
		out[tone] = true
	}
	return out
}()

// ToneColor is the color a tone paints; false for a name that is not a tone.
func ToneColor(name string) (Color, bool) {
	hue, ok := toneHues[name]
	if !ok {
		return Color{}, false
	}
	c, err := ParseColor(hue, nil)
	return c, err == nil
}

// operatorColorNameRe is the grammar of a `palette.colors` name. The name
// becomes part of a CSS custom property name and a style class, so it is an
// allowlist of characters, not a blocklist.
var operatorColorNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// opaqueHexRe accepts #rgb and #rrggbb. Operator colors take no alpha: a
// translucent fill blends with what is behind it, so the text color chosen
// for it could not be shown to reach WCAG AA.
var opaqueHexRe = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// ColorKind tells the three color name forms apart.
type ColorKind int

// The color name forms.
const (
	ColorBare ColorKind = iota + 1
	ColorShade
	ColorOperator
)

// Color is a parsed color name.
type Color struct {
	Kind ColorKind
	// Hue is the canonical hue (`grey`, never `gray`) for a bare or shade
	// color; empty for an operator color.
	Hue string
	// Shade is the kit step (`700`) of a shade color; empty otherwise.
	Shade string
	// Name is the operator color's name; empty otherwise.
	Name string
}

// ParseColor parses a color name against the kit and the operator colors.
// operator may be nil.
func ParseColor(name string, operator map[string]OperatorColor) (Color, error) {
	if hue, shade, ok := strings.Cut(name, "/"); ok {
		hue = canonicalHue(hue)
		if _, ok := kitRamps[hue]; !ok {
			if bareHues[hue] {
				return Color{}, fmt.Errorf("unknown color %q: %q has no shades (%s)", name, hue, colorForms())
			}
			return Color{}, fmt.Errorf("unknown color %q: %q is not a hue (%s)", name, hue, colorForms())
		}
		if !slices.Contains(kitShades, shade) {
			return Color{}, fmt.Errorf("unknown color %q: shade must be one of %s",
				name, strings.Join(kitShades, ", "))
		}
		return Color{Kind: ColorShade, Hue: hue, Shade: shade}, nil
	}
	if hue := canonicalHue(name); bareHues[hue] {
		return Color{Kind: ColorBare, Hue: hue}, nil
	}
	if _, ok := operator[name]; ok {
		return Color{Kind: ColorOperator, Name: name}, nil
	}
	return Color{}, fmt.Errorf("unknown color %q (%s)", name, colorForms())
}

func canonicalHue(h string) string {
	if c, ok := hueAliases[h]; ok {
		return c
	}
	return h
}

// colorForms describes the valid color names for an error message.
func colorForms() string {
	return "valid: grey, red, orange, yellow, green, teal, blue, purple; " +
		"a shade such as red/700 (50-900, not purple); or a name under palette.colors"
}

// StyleClass is the class the SPA receives in the styles map for this color:
// `badge-<hue>` for a bare hue (`badge-gray` for grey, the class it always
// had), `kit-<hue>-<shade>` for a shade and `custom-<name>` for an operator
// color. frontend/src/utils/styleColors.ts resolves each form to tokens.
func (c Color) StyleClass() string {
	switch c.Kind {
	case ColorShade:
		return "kit-" + c.Hue + "-" + c.Shade
	case ColorOperator:
		return "custom-" + c.Name
	default:
		if c.Hue == "grey" {
			return "badge-gray"
		}
		return "badge-" + c.Hue
	}
}

// OperatorColor is a project color under `palette.colors`. Dark is optional
// and falls back to Light.
type OperatorColor struct {
	Light string `yaml:"light" json:"light"`
	Dark  string `yaml:"dark,omitempty" json:"dark,omitempty"`
}

// validateOperatorColors checks `palette.colors`.
func validateOperatorColors(colors map[string]OperatorColor) error {
	for _, name := range sortedMapKeys(colors) {
		c := colors[name]
		if !operatorColorNameRe.MatchString(name) {
			return fmt.Errorf("colors: name %q must be lower-case letters, digits and hyphens, starting with a letter", name)
		}
		if bareHues[canonicalHue(name)] || SemanticTones[name] {
			return fmt.Errorf("colors: name %q is reserved for a kit color or tone", name)
		}
		// Each name also claims the token `<name>-fg` for its text color.
		if strings.HasSuffix(name, "-fg") {
			return fmt.Errorf("colors: name %q must not end in -fg", name)
		}
		if c.Light == "" {
			return fmt.Errorf("colors.%s: light is required", name)
		}
		if !opaqueHexRe.MatchString(c.Light) {
			return fmt.Errorf("colors.%s.light: invalid color %q (expected #rgb or #rrggbb)", name, c.Light)
		}
		if c.Dark != "" && !opaqueHexRe.MatchString(c.Dark) {
			return fmt.Errorf("colors.%s.dark: invalid color %q (expected #rgb or #rrggbb)", name, c.Dark)
		}
	}
	return nil
}

// validateDarkKitOverrides checks `palette.dark.colors`: kit shades only, with
// an opaque hex. An operator color sets its dark value on itself.
func validateDarkKitOverrides(colors map[string]string) error {
	for _, name := range sortedMapKeys(colors) {
		c, err := ParseColor(name, nil)
		if err != nil || c.Kind != ColorShade {
			return fmt.Errorf("colors: %q is not a kit shade such as red/700", name)
		}
		if !opaqueHexRe.MatchString(colors[name]) {
			return fmt.Errorf("colors.%s: invalid color %q (expected #rgb or #rrggbb)", name, colors[name])
		}
	}
	return nil
}

// kitHex returns the light value of a kit shade.
func kitHex(c Color) string {
	return kitRamps[c.Hue][slices.Index(kitShades, c.Shade)]
}

// kitToken is the CSS custom property that paints a kit shade.
func kitToken(c Color) string {
	return "--rl-kit-" + c.Hue + "-" + c.Shade
}

// operatorToken is the CSS custom property that paints an operator color.
func operatorToken(name string) string {
	return "--rl-custom-" + name
}

// Text colors ForegroundFor chooses between. darkText is kit grey/900, the
// palette's body text.
const (
	whiteText = "#ffffff"
	darkText  = "#25272d"
	blackText = "#000000"
	// aaText is the WCAG AA minimum contrast for normal text.
	aaText = 4.5
)

// ForegroundFor returns a text color that reaches WCAG AA (4.5:1) on bg:
// white or kit grey/900, whichever contrasts more, or black when neither
// reaches AA. One of white and black always does: their contrasts with any
// color multiply to 21, so the larger is at least sqrt(21) = 4.58.
func ForegroundFor(bg string) string {
	w := contrastRatio(bg, whiteText)
	d := contrastRatio(bg, darkText)
	if math.Max(w, d) >= aaText {
		if w >= d {
			return whiteText
		}
		return darkText
	}
	if w >= contrastRatio(bg, blackText) {
		return whiteText
	}
	return blackText
}

// contrastRatio is the WCAG 2 contrast ratio of two colors.
func contrastRatio(a, b string) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	const flare = 0.05
	return (la + flare) / (lb + flare)
}

// relativeLuminance is the WCAG 2 relative luminance of a hex color.
func relativeLuminance(hex string) float64 {
	r, g, b := hexToRGB(hex)
	lin := func(c float64) float64 {
		const (
			knee  = 0.03928
			slope = 12.92
			off   = 0.055
			gamma = 2.4
		)
		if c <= knee {
			return c / slope
		}
		return math.Pow((c+off)/(1+off), gamma)
	}
	const wr, wg, wb = 0.2126, 0.7152, 0.0722
	return wr*lin(r) + wg*lin(g) + wb*lin(b)
}

// OperatorColors returns the project's `palette.colors`, nil without a palette.
func (c *Config) OperatorColors() map[string]OperatorColor {
	if c.Palette == nil {
		return nil
	}
	return c.Palette.Colors
}

// validatePaletteConfig checks the project palette at load, so a bad hex or an
// unknown color name fails the config rather than painting nothing.
func validatePaletteConfig(cfg *Config) []string {
	if err := ValidatePalette(cfg.Palette); err != nil {
		return []string{err.Error()}
	}
	return nil
}
