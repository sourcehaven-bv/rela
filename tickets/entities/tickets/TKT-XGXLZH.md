---
id: TKT-XGXLZH
type: ticket
title: Serve the resolved project palette in _rela.css (not framework defaults)
kind: enhancement
priority: medium
effort: s
status: done
---

The per-app stylesheet served at `/api/v1/_apps/<id>/_rela.css` (`appCSSSource`)
emitted rela's **default** theme tokens (the embedded `apps_tokens.css`
`:root`/`:root.dark` blocks), ignoring the project's resolved palette
(`.rela/palette.yaml`). A custom app that opts in with
`<link href="_rela.css">` therefore rendered with framework default colors
(cream `#f3f2ef`, blue accent) instead of the host project's actual theme.

The SPA derives 21 CSS vars from the 8 palette colors
(`dataentryconfig.deriveTheme`), so the app *shell* looked right but embedded
apps drifted. The only app-side workaround was to port `deriveTheme` into the
app's JS — ~80 lines duplicating `frontend/src/utils/palette.ts`, exactly the
frozen-contract drift `internal/dataentry/CLAUDE.md` warns against.

Follow-up to TKT-F5FDEQ (which shipped `_rela.css`); implements FEAT-BFDB9Q.

## Fix

**STATUS: done.** `appCSSSource(palette *ResolvedPalette)` now renders `:root`
from `palette.Light` and `:root.dark` from `palette.Dark` (deterministic, sorted
keys), then appends the unchanged atomic controls. `handleV1App` passes
`a.State().Palette`. Falls back to the embedded default tokens when
`palette == nil` — and because `deriveTheme`'s output keys are exactly the 21
vars in `apps_tokens.css`, a `nil`/default palette is value-equivalent to the
previous embed.

## Tests

- `TestAppTokensCSSInSyncWithFrontend` unchanged — still pins the embedded
  fallback to the SPA source of truth.
- `TestAppCSSSource(nil)` — fallback path still carries `:root`, `:root.dark`,
  and the atomic controls; still rejects component-shaped classes.
- `TestAppCSSSourceUsesResolvedPalette` (new) — a configured project palette
  appears in the served CSS and the default surface `#f3f2ef` does not leak.
