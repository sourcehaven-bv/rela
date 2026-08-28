---
id: TKT-8AJ1PM
type: ticket
title: Extract dataentry theme/settings/palette cluster to appearanceHandler (App 104 → 92)
kind: refactor
priority: medium
effort: m
status: done
---

Sub-ticket of the [[TKT-R68TV8]] `dataentry.App` decomposition arc.

## What

**Pure structural extraction, no behavior change.** Move the 12 theme/settings/
palette/logo glue methods off `App` into a new `appearanceHandler`:

- `handlers_theme.go` (4): `handleAPIThemeLogo`, `handleAPIGetThemeLogo`,
`handleAPIPutThemeLogo`, `handleAPIDeleteThemeLogo`
- `handlers_theme_package.go` (2): `handleAPIThemeExport`, `handleAPIThemeImport`
- `settings_handlers.go` (6): `handleAPISettingsCRUD`, `handleAPIGetSettings`,
`handleAPISaveSettings`, `handleAPIPaletteCRUD`, `handleAPIGetPalette`,
`handleAPISavePalette`

All three collaborators (`logo *logoStore`, `palette *paletteService`, `settings
*settingsService`) are already extracted self-synchronized services — App's
methods are pure glue. No store writes, no writeMu, no ACL decisions in this
cluster (the one relation-default lookup in `handleAPIGetSettings` keeps its
existing read path).

Collaborator shape mirrors `viewsHandler`: fixed service handles by value,
reloadable state (`schema func() *Schema`, `services func() Services`) as
closures. Wiring parity NewApp ↔ rebindApp. Routes re-point via
`mux.HandleFunc(path, a.appearance.X)` so the mount costs App zero methods.

Ratchet `//plimsoll:max-methods` on App 104 → 92.

## Done when

plimsoll passes with the lowered directive, full test suite + race on dataentry
green, arch-lint/comment-lint/golangci-lint clean.
