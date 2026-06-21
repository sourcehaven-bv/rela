package dataentry

import _ "embed"

// appTokensCSS is the rela theme-token stylesheet (:root / :root.dark custom
// properties), embedded from a copy of the SPA's source of truth
// (frontend/src/styles/tokens.css). The copy is kept byte-identical by
// TestAppTokensCSSInSyncWithFrontend so the SPA and the app stylesheet can
// never drift. Do not hand-edit apps_tokens.css; edit the frontend source and
// re-copy.
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
// /api/v1/_apps/<id>/_rela.css: the shared theme tokens plus the base controls.
// An app opts in with <link rel="stylesheet" href="_rela.css">; theme follows
// the host because the SDK toggles `dark` on the app's <html> (the tokens use
// the same `:root.dark` selector as the SPA).
func appCSSSource() string {
	return appTokensCSS + "\n" + appBaseControlsCSS
}
