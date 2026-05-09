package dataentryconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// hexColorRe validates hex color formats: #rgb, #rrggbb, #rrggbbaa.
var hexColorRe = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// ValidBadgeNames is the set of allowed badge color names.
var ValidBadgeNames = map[string]bool{
	"blue": true, "purple": true, "green": true, "gray": true,
	"red": true, "orange": true, "yellow": true,
}

// PaletteConfig is the top-level palette configuration in data-entry.yaml.
type PaletteConfig struct {
	PaletteColors `yaml:",inline"`
	Badges        map[string]string `yaml:"badges,omitempty" json:"badges,omitempty"`
	Dark          DarkMode          `yaml:"dark,omitempty"   json:"dark,omitempty"`
}

// PaletteColors holds the 8 named color roles. All fields are optional;
// unset fields fall back to built-in defaults.
type PaletteColors struct {
	Base    string `yaml:"base,omitempty"    json:"base,omitempty"`
	Surface string `yaml:"surface,omitempty" json:"surface,omitempty"`
	Accent  string `yaml:"accent,omitempty"  json:"accent,omitempty"`
	Text    string `yaml:"text,omitempty"    json:"text,omitempty"`
	Success string `yaml:"success,omitempty" json:"success,omitempty"`
	Error   string `yaml:"error,omitempty"   json:"error,omitempty"`
	Warning string `yaml:"warning,omitempty" json:"warning,omitempty"`
	Info    string `yaml:"info,omitempty"    json:"info,omitempty"`
}

// DarkPalette is the explicit dark-mode payload. It carries the same
// 8 base color roles as the light palette plus an optional dark badge
// override map. Empty fields inherit from the resolved light palette.
type DarkPalette struct {
	PaletteColors `yaml:",inline"`
	Badges        map[string]string `yaml:"badges,omitempty" json:"badges,omitempty"`
}

// DarkMode represents the dark mode setting: either disabled (`false`)
// or an explicit DarkPalette object. There is no `auto` mode at
// runtime — auto-derivation is performed in the frontend Settings UI
// via an explicit Derive button which writes a fully-populated dark
// palette into this field.
type DarkMode struct {
	Disabled bool         // true when dark mode is turned off (`dark: false`)
	Explicit *DarkPalette // non-nil when an explicit palette is provided
}

// IsDisabled returns true if dark mode is disabled.
func (d DarkMode) IsDisabled() bool {
	return d.Disabled
}

// IsExplicit returns true if an explicit dark palette was provided.
func (d DarkMode) IsExplicit() bool {
	return d.Explicit != nil
}

// UnmarshalYAML handles the two-way union: bool false or an object.
func (d *DarkMode) UnmarshalYAML(value *yaml.Node) error {
	// Try bool (only `false` is meaningful; `true` is rejected).
	var boolVal bool
	if err := value.Decode(&boolVal); err == nil {
		if boolVal {
			return errors.New("invalid dark mode `true`: use either `false` or an explicit palette object")
		}
		d.Disabled = true
		return nil
	}

	// Try object (explicit palette with optional dark badges).
	var dp DarkPalette
	if err := value.Decode(&dp); err != nil {
		return fmt.Errorf("invalid dark mode: must be `false` or a palette object: %w", err)
	}
	d.Explicit = &dp
	return nil
}

// MarshalYAML serializes DarkMode back to YAML.
func (d DarkMode) MarshalYAML() (interface{}, error) {
	if d.Explicit != nil {
		return d.Explicit, nil
	}
	if d.Disabled {
		return false, nil
	}
	// Zero value → emit explicit YAML null. Reachable when a parent
	// struct without `omitempty` (or with a non-effective omitempty
	// for custom marshalers) decides to include the field.
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
}

// MarshalJSON serializes DarkMode to JSON. Returns `false` or the
// explicit PaletteColors object. A zero-value DarkMode marshals to
// `null` (omitted in `omitempty` contexts).
func (d DarkMode) MarshalJSON() ([]byte, error) {
	if d.Explicit != nil {
		return json.Marshal(d.Explicit)
	}
	if d.Disabled {
		return json.Marshal(false)
	}
	return []byte("null"), nil
}

// UnmarshalJSON handles the two-way union for JSON: bool false or
// an object.
func (d *DarkMode) UnmarshalJSON(data []byte) error {
	// Allow JSON null → zero-value DarkMode.
	if string(data) == "null" {
		return nil
	}

	// Try bool (only `false`; `true` is rejected).
	var boolVal bool
	if err := json.Unmarshal(data, &boolVal); err == nil {
		if boolVal {
			return errors.New("invalid dark mode `true`: use either `false` or an explicit palette object")
		}
		d.Disabled = true
		return nil
	}

	// Try object (explicit palette with optional dark badges).
	var dp DarkPalette
	if err := json.Unmarshal(data, &dp); err != nil {
		return fmt.Errorf("invalid dark mode: must be `false` or a palette object: %w", err)
	}
	d.Explicit = &dp
	return nil
}

// ResolvedPalette contains the fully resolved CSS variable values for both themes.
type ResolvedPalette struct {
	Light        map[string]string `json:"light"`
	Dark         map[string]string `json:"dark,omitempty"`
	DarkDisabled bool              `json:"darkDisabled,omitempty"`
}

// Built-in default colors for light mode. Derived from sourcehaven.nl —
// cream paper surface, harbor-navy sidebar, blue accent.
var defaultLightColors = PaletteColors{
	Base:    "#164155",
	Surface: "#f3f2ef",
	Accent:  "#4772fb",
	Text:    "#191919",
	Success: "#06ce90",
	Error:   "#e5484d",
	Warning: "#f4aa83",
	Info:    "#4772fb",
}

// Built-in default colors for dark mode. Same hue family as the light
// defaults — harbor-navy surface, cream-tinted text, lifted blue accent.
var defaultDarkColors = PaletteColors{
	Base:    "#091821",
	Surface: "#0f1f28",
	Accent:  "#6f93ff",
	Text:    "#ece9e0",
	Success: "#34d39c",
	Error:   "#f87171",
	Warning: "#f9d975",
	Info:    "#6f93ff",
}

// Built-in default badge colors.
var defaultBadgeColors = map[string]string{
	"blue":   "#4772fb",
	"purple": "#8b5cf6",
	"green":  "#06ce90",
	"gray":   "#6b7280",
	"red":    "#e5484d",
	"orange": "#f4aa83",
	"yellow": "#f9d975",
}

// Built-in default badge colors for dark mode.
var defaultDarkBadges = map[string]string{
	"blue":   "#6f93ff",
	"purple": "#c4b5fd",
	"green":  "#34d39c",
	"gray":   "#8fa4b0",
	"red":    "#f87171",
	"orange": "#f4aa83",
	"yellow": "#f9d975",
}

// ValidateHexColor checks that a string is a valid hex color.
func ValidateHexColor(s string) error {
	if !hexColorRe.MatchString(s) {
		return fmt.Errorf("invalid hex color %q (expected #rgb, #rrggbb, or #rrggbbaa)", s)
	}
	return nil
}

// ValidatePalette validates all color values in a PaletteConfig.
func ValidatePalette(p *PaletteConfig) error {
	if p == nil {
		return nil
	}
	if err := validateColors(&p.PaletteColors); err != nil {
		return fmt.Errorf("palette: %w", err)
	}
	for name, color := range p.Badges {
		if !ValidBadgeNames[name] {
			return fmt.Errorf("palette.badges: unknown badge color %q", name)
		}
		if err := ValidateHexColor(color); err != nil {
			return fmt.Errorf("palette.badges.%s: %w", name, err)
		}
	}
	if p.Dark.IsExplicit() {
		if err := validateColors(&p.Dark.Explicit.PaletteColors); err != nil {
			return fmt.Errorf("palette.dark: %w", err)
		}
		for name, color := range p.Dark.Explicit.Badges {
			if !ValidBadgeNames[name] {
				return fmt.Errorf("palette.dark.badges: unknown badge color %q", name)
			}
			if err := ValidateHexColor(color); err != nil {
				return fmt.Errorf("palette.dark.badges.%s: %w", name, err)
			}
		}
	}
	return nil
}

func validateColors(c *PaletteColors) error {
	fields := map[string]string{
		"base": c.Base, "surface": c.Surface, "accent": c.Accent, "text": c.Text,
		"success": c.Success, "error": c.Error, "warning": c.Warning, "info": c.Info,
	}
	for name, val := range fields {
		if val == "" {
			continue
		}
		if err := ValidateHexColor(val); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

// mergeColors fills empty fields in dst with values from src.
func mergeColors(dst, src *PaletteColors) {
	if dst.Base == "" {
		dst.Base = src.Base
	}
	if dst.Surface == "" {
		dst.Surface = src.Surface
	}
	if dst.Accent == "" {
		dst.Accent = src.Accent
	}
	if dst.Text == "" {
		dst.Text = src.Text
	}
	if dst.Success == "" {
		dst.Success = src.Success
	}
	if dst.Error == "" {
		dst.Error = src.Error
	}
	if dst.Warning == "" {
		dst.Warning = src.Warning
	}
	if dst.Info == "" {
		dst.Info = src.Info
	}
}

// ResolvePalette merges project and user palettes with defaults and
// produces a ResolvedPalette containing the fully derived CSS
// variable map.
//
// Dark mode is on by default — when neither project nor user supplied
// a dark configuration, the built-in defaultDarkColors palette is
// used. To turn dark mode off, a project or user palette must
// explicitly set `dark: false`. Empty fields in an explicit dark
// palette inherit from the resolved light palette so that partial
// overrides compose cleanly with the user's chosen light theme.
func ResolvePalette(project, user *PaletteConfig) *ResolvedPalette {
	// Start with defaults.
	light := defaultLightColors
	badges := copyBadges(defaultBadgeColors)

	// Layer project palette.
	if project != nil {
		light = applyOver(light, project.PaletteColors)
		mergeBadges(badges, project.Badges)
	}

	// Layer user palette (highest priority).
	if user != nil {
		light = applyOver(light, user.PaletteColors)
		mergeBadges(badges, user.Badges)
	}

	// Determine dark mode. The user palette wins if it specifies
	// anything; otherwise the project palette; otherwise dark mode
	// uses the built-in default dark palette.
	darkMode := DarkMode{Explicit: &DarkPalette{
		PaletteColors: defaultDarkColors,
		Badges:        copyBadges(defaultDarkBadges),
	}}
	if project != nil && (project.Dark.Disabled || project.Dark.Explicit != nil) {
		darkMode = project.Dark
	}
	if user != nil && (user.Dark.Disabled || user.Dark.Explicit != nil) {
		darkMode = user.Dark
	}

	result := &ResolvedPalette{
		Light: deriveTheme(light, badges),
	}

	if darkMode.IsDisabled() {
		result.DarkDisabled = true
		return result
	}

	// Defensive: a zero-value DarkMode (e.g. from a JSON `null`
	// payload) would reach here without Explicit set. Treat that as
	// disabled rather than panicking on a nil dereference.
	if darkMode.Explicit == nil {
		result.DarkDisabled = true
		return result
	}

	// Explicit dark palette: empty fields inherit from light so that
	// partial overrides compose with the user's chosen light theme.
	// Same rule for dark badges: unset entries fall back to the
	// resolved light badges.
	darkColors := darkMode.Explicit.PaletteColors
	mergeColors(&darkColors, &light)
	darkBadges := copyBadges(badges)
	mergeBadges(darkBadges, darkMode.Explicit.Badges)
	result.Dark = deriveTheme(darkColors, darkBadges)

	return result
}

// applyOver returns base with non-empty fields from over applied.
func applyOver(base, over PaletteColors) PaletteColors {
	if over.Base != "" {
		base.Base = over.Base
	}
	if over.Surface != "" {
		base.Surface = over.Surface
	}
	if over.Accent != "" {
		base.Accent = over.Accent
	}
	if over.Text != "" {
		base.Text = over.Text
	}
	if over.Success != "" {
		base.Success = over.Success
	}
	if over.Error != "" {
		base.Error = over.Error
	}
	if over.Warning != "" {
		base.Warning = over.Warning
	}
	if over.Info != "" {
		base.Info = over.Info
	}
	return base
}

func copyBadges(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func mergeBadges(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}

// deriveTheme produces the full 21-variable CSS map from 8 base colors + 7 badges.
func deriveTheme(colors PaletteColors, badges map[string]string) map[string]string {
	m := map[string]string{
		"--sidebar-bg":    colors.Base,
		"--bg-color":      colors.Surface,
		"--accent-color":  colors.Accent,
		"--text-color":    colors.Text,
		"--success-color": colors.Success,
		"--error-color":   colors.Error,
		"--warning-color": colors.Warning,
		"--info-color":    colors.Info,
	}

	// Derive 6 computed variables
	surfaceHSL := hexToHSL(colors.Surface)
	textHSL := hexToHSL(colors.Text)
	baseHSL := hexToHSL(colors.Base)

	// card-bg: lighten surface slightly
	if surfaceHSL.L >= deriveLightClamp {
		m["--card-bg"] = colors.Surface
	} else {
		m["--card-bg"] = hslToHex(hsl{surfaceHSL.H, surfaceHSL.S, clamp01(surfaceHSL.L + deriveLighten)})
	}

	// input-bg: same as card-bg
	m["--input-bg"] = m["--card-bg"]

	// hover-bg: darken surface slightly
	if surfaceHSL.L <= deriveDarkClamp {
		m["--hover-bg"] = colors.Surface
	} else {
		m["--hover-bg"] = hslToHex(hsl{surfaceHSL.H, surfaceHSL.S, clamp01(surfaceHSL.L - deriveDarken)})
	}

	// border-color: mix surface and text
	m["--border-color"] = mixColors(colors.Surface, colors.Text, deriveBorderMix)

	// muted-text: lighten text
	m["--muted-text"] = hslToHex(hsl{textHSL.H, textHSL.S, clamp01(textHSL.L + deriveMutedShift)})

	// sidebar-text: high contrast against base
	if baseHSL.L < lightnessThreshold {
		m["--sidebar-text"] = "#e8e8e8"
	} else {
		m["--sidebar-text"] = "#1e293b"
	}

	// Badge colors
	for name, color := range badges {
		m["--badge-"+name] = color
	}

	return m
}

// generateDark creates a dark palette from a light palette by
// inverting lightness.
//
// This function is the *reference implementation* for dark
// derivation. It is no longer called from production code paths —
// dark derivation is performed in the frontend
// (frontend/src/utils/palette.ts: generateDark) so users can preview
// it via the "Derive Dark from Light" button before saving. This Go
// version is kept solely as the source of truth for the parity
// goldens generated by TestGenerateDarkParityGoldens (see
// palette_parity_test.go), which the TS test consumes verbatim.
//
// If you change the algorithm here, regenerate the goldens with
// `UPDATE_GOLDENS=1 go test ./internal/dataentryconfig/ -run TestGenerateDarkParityGoldens`
// and re-run the frontend palette test suite to confirm parity.
func generateDark(light PaletteColors) PaletteColors {
	return PaletteColors{
		Base:    adjustLightness(light.Base, darkBaseDelta),
		Surface: invertLightness(light.Surface, darkSurfaceTarget),
		Accent:  adjustLightness(light.Accent, darkBrightenDelta),
		Text:    invertLightness(light.Text, darkTextTarget),
		Success: adjustLightness(light.Success, darkBrightenDelta),
		Error:   adjustLightness(light.Error, darkBrightenDelta),
		Warning: adjustLightness(light.Warning, darkBrightenDelta),
		Info:    adjustLightness(light.Info, darkBrightenDelta),
	}
}

// generateDarkBadges slightly brightens badge colors for dark mode
// visibility. Like generateDark, this is the reference impl kept
// only for the parity goldens — see generateDark's comment.
func generateDarkBadges(badges map[string]string) map[string]string {
	dark := make(map[string]string, len(badges))
	for name, color := range badges {
		dark[name] = adjustLightness(color, darkBrightenDelta)
	}
	return dark
}

// --- HSL Color Utilities ---

// Color manipulation constants.
const (
	// Derivation deltas for computed CSS variables.
	deriveLighten    = 0.02 // card-bg/input-bg lightness increase
	deriveDarken     = 0.03 // hover-bg lightness decrease
	deriveBorderMix  = 0.15 // border-color mix ratio (surface+text)
	deriveMutedShift = 0.30 // muted-text lightness increase
	deriveLightClamp = 0.98 // surface lightness above which card-bg clamps
	deriveDarkClamp  = 0.03 // surface lightness below which hover-bg clamps

	// Dark mode generation constants.
	darkBrightenDelta  = 0.10
	darkSurfaceTarget  = 0.08
	darkTextTarget     = 0.85
	darkBaseDelta      = -0.05
	lightnessThreshold = 0.5

	hexAlphaLen = 8
	hexShortLen = 3
)

type hsl struct {
	H, S, L float64
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// normalizeHex expands 3-digit hex to 6-digit and strips alpha channel.
func normalizeHex(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == hexShortLen {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) == hexAlphaLen {
		hex = hex[:6] // strip alpha
	}
	return "#" + hex
}

func hexToRGB(hex string) (r, g, b float64) {
	hex = strings.TrimPrefix(normalizeHex(hex), "#")
	var ri, gi, bi uint64
	_, _ = fmt.Sscanf(hex, "%02x%02x%02x", &ri, &gi, &bi)
	return float64(ri) / rgbMax, float64(gi) / rgbMax, float64(bi) / rgbMax
}

const rgbMax = 255

func rgbToHex(r, g, b float64) string {
	ri := int(math.Round(r * rgbMax))
	gi := int(math.Round(g * rgbMax))
	bi := int(math.Round(b * rgbMax))
	return fmt.Sprintf("#%02x%02x%02x", ri, gi, bi)
}

func hexToHSL(hex string) hsl {
	r, g, b := hexToRGB(hex)
	maxC := math.Max(r, math.Max(g, b))
	minC := math.Min(r, math.Min(g, b))
	l := (maxC + minC) / 2

	if maxC == minC {
		return hsl{0, 0, l}
	}

	d := maxC - minC
	const half = 0.5
	var s float64
	if l > half {
		s = d / (2 - maxC - minC)
	} else {
		s = d / (maxC + minC)
	}

	var h float64
	switch maxC {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	case b:
		h = (r-g)/d + 4
	}
	h /= 6

	return hsl{h, s, l}
}

func hslToHex(c hsl) string {
	r, g, b := hslToRGB(c)
	return rgbToHex(r, g, b)
}

func hslToRGB(c hsl) (r, g, b float64) {
	if c.S == 0 {
		return c.L, c.L, c.L
	}

	const half = 0.5
	var q float64
	if c.L < half {
		q = c.L * (1 + c.S)
	} else {
		q = c.L + c.S - c.L*c.S
	}
	p := 2*c.L - q

	r = hueToRGB(p, q, c.H+1.0/3.0)
	g = hueToRGB(p, q, c.H)
	b = hueToRGB(p, q, c.H-1.0/3.0)
	return r, g, b
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

// mixColors blends two hex colors. ratio=0 returns color1, ratio=1 returns color2.
func mixColors(hex1, hex2 string, ratio float64) string {
	r1, g1, b1 := hexToRGB(hex1)
	r2, g2, b2 := hexToRGB(hex2)
	r := r1*(1-ratio) + r2*ratio
	g := g1*(1-ratio) + g2*ratio
	b := b1*(1-ratio) + b2*ratio
	return rgbToHex(r, g, b)
}

// adjustLightness shifts the lightness of a hex color by delta (clamped to [0,1]).
func adjustLightness(hex string, delta float64) string {
	c := hexToHSL(hex)
	c.L = clamp01(c.L + delta)
	return hslToHex(c)
}

// invertLightness maps a color's lightness to targetL (for dark mode generation).
func invertLightness(hex string, targetL float64) string {
	c := hexToHSL(hex)
	c.L = clamp01(targetL)
	return hslToHex(c)
}
