package dataentry

import (
	_ "embed"
	"slices"
	"strings"
)

// appTokensCSS is the rela theme-token stylesheet (:root / :root.dark custom
// properties), embedded from a copy of the SPA's source of truth
// (frontend/src/styles/tokens.css). The copy is kept byte-identical by
// TestAppTokensCSSInSyncWithFrontend so the SPA and the app stylesheet can
// never drift. Do not hand-edit apps_tokens.css; edit the frontend source and
// re-copy.
//
// This embed is the FALLBACK only: appCSSSource renders the project's resolved
// palette when one is supplied (the common case), and falls back to these
// default tokens when palette is nil.
//
//go:embed apps_tokens.css
var appTokensCSS string

// appBaseControlsCSS is the small set of atomic, frozen-contract controls apps
// may opt into (in addition to the tokens above). Intentionally tiny: only
// pure-presentation elements with one obvious HTML shape — button, bare text
// input, card. Anything component-shaped (selects, tables, modals, pickers)
// stays out; apps build those from the tokens. These selectors are an
// app-facing contract — change their appearance, not their meaning.
const appBaseControlsCSS = `
/* --- rela base controls (opt-in, via <link href="_rela.css">) --- */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.45rem 0.9rem;
  font: inherit;
  line-height: 1.2;
  color: var(--text-color);
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  cursor: pointer;
}
.btn:hover { background: var(--hover-bg); }
.btn:disabled { opacity: 0.55; cursor: default; }
.btn-primary {
  color: #fff;
  background: var(--accent-color);
  border-color: var(--accent-color);
}
.btn-primary:hover { filter: brightness(1.05); background: var(--accent-color); }
.input {
  display: block;
  width: 100%;
  padding: 0.45rem 0.6rem;
  font: inherit;
  color: var(--text-color);
  background: var(--input-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
}
.input:focus {
  outline: none;
  border-color: var(--accent-color);
}
.card {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 1rem;
}
`

// appTypographyCSS defines the font tokens and applies them on the app document.
// Unlike the color tokens, typography is palette-INDEPENDENT (it's the same for
// every project), so it is emitted unconditionally by appCSSSource — the
// per-project palette path only rewrites the color :root block and would
// otherwise leave --font-family undefined. Applying family + base size on <html>
// is what stops a linking app from falling back to the browser default serif at
// 16px (a sandboxed app iframe is a separate document with no inherited
// typography). Apps can override per-element. Font is app-only (the SPA sets its
// own in App.vue), so it lives here rather than in the shared tokens.css.
const appTypographyCSS = `
/* --- rela typography (font tokens + application) --- */
:root {
  --font-family: 'Open Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto,
    'Helvetica Neue', Arial, sans-serif;
  --font-size-sm: 12px;
  --font-size-base: 14px;
  --font-size-lg: 18px;
  --font-size-xl: 22px;
}
html {
  font-family: var(--font-family);
  font-size: var(--font-size-base);
}
`

// appCSSSource returns the stylesheet served at /api/v1/_apps/<id>/_rela.css:
// the theme tokens followed by the base controls. Theme follows the host via
// the `:root.dark` selector the SDK toggles. A non-nil palette renders the
// token blocks from the project's resolved palette, so apps match the host;
// nil uses the embedded default tokens (apps_tokens.css).
func appCSSSource(palette *ResolvedPalette) string {
	var b strings.Builder
	if palette != nil && len(palette.Light) > 0 {
		b.WriteString(renderTokenBlock(":root", palette.Light))
		if !palette.DarkDisabled && len(palette.Dark) > 0 {
			b.WriteString("\n")
			b.WriteString(renderTokenBlock(":root.dark", palette.Dark))
		}
	} else {
		b.WriteString(appTokensCSS)
	}
	b.WriteString("\n")
	b.WriteString(appTypographyCSS)
	b.WriteString("\n")
	b.WriteString(appBaseControlsCSS)
	return b.String()
}

// renderTokenBlock formats a `selector { --var: value; ... }` block with the
// custom properties sorted by name, so the output is deterministic (stable
// across reloads and testable). Values come from the resolved palette and are
// already validated hex strings.
func renderTokenBlock(selector string, vars map[string]string) string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var b strings.Builder
	b.WriteString(selector)
	b.WriteString(" {\n")
	for _, k := range keys {
		b.WriteString("  ")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(vars[k])
		b.WriteString(";\n")
	}
	b.WriteString("}\n")
	return b.String()
}
