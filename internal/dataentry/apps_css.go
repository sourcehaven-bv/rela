package dataentry

import (
	_ "embed"
	"sort"
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

// appCSSSource returns the full stylesheet served at the reserved per-app path
// /api/v1/_apps/<id>/_rela.css: the theme tokens followed by the base controls.
// An app opts in with <link rel="stylesheet" href="_rela.css">; theme follows
// the host because the SDK toggles `dark` on the app's <html> (the tokens use
// the same `:root.dark` selector as the SPA).
//
// When palette is non-nil, the :root / :root.dark token blocks are rendered
// from the PROJECT's resolved palette — the same 21 CSS variables the SPA
// derives (dataentryconfig.deriveTheme) and serves at /_palette — so an app
// matches the host's actual theme rather than the framework defaults. This is
// what keeps every custom app from having to re-derive the palette itself.
// When palette is nil, the embedded default tokens (apps_tokens.css) are used.
func appCSSSource(palette *ResolvedPalette) string {
	var b strings.Builder
	if palette != nil && len(palette.Light) > 0 {
		b.WriteString(renderTokenBlock(":root", palette.Light))
		if !palette.DarkDisabled && len(palette.Dark) > 0 {
			b.WriteString("\n")
			b.WriteString(renderTokenBlock(":root.dark", palette.Dark))
		}
	} else {
		// No resolved palette — fall back to the embedded default tokens.
		b.WriteString(appTokensCSS)
	}
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
	sort.Strings(keys)

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
