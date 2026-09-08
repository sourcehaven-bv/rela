---
id: TKT-XDJTDC
type: ticket
title: 'Extract configAPI off dataentry.App: the principal-independent metadata surface (87 → ~80)'
kind: refactor
priority: medium
effort: m
tags:
    - tech-debt
status: backlog
---

Sub-ticket of [[TKT-R68TV8]] (the `dataentry.App` arc under [[TKT-N0IKN9]]).
Recommended FIRST App PR: smallest blast radius, and it establishes the
constructor pattern the following three extractions copy.

## Why this is an abstraction, not a file move

CLAUDE.md's rule "the configuration is not a secret; the data is" is prose
today. Every handler in this cluster serves operator-authored config
byte-identically to every principal, touches no store, and takes no read gate.
Making that a type turns the rule into structure: `configAPI` has **no store, no
reader, no ACL field**, so a reviewer can see at a glance that no entity data
can leak through it, and a change that needs one has to justify adding the
field.

## What moves (7 methods, `internal/dataentry/`)

- `handleV1Schema` (api_v1.go:1282), `handleV1SchemaRoutes` (:1337),
`handleV1Config` (:1566), `handleV1OpenAPI` (:2653), `handleV1Templates`
(:2608), `handleV1App` (apps_handler.go:88).
- `scanAppsOrLog` (apps_handler.go:209) becomes a package function taking
`root string`.

Field set used (verified): `{schema, palette, templater, paths, versions}`.
`templater` and `versions` leave `App` entirely.

## Shape

```go
// configAPI serves the principal-independent metadata surface: /_schema,
// /_schema/*, /_config, /_openapi.json, /_templates/*, /_apps/*.
// It deliberately holds NO store, NO reader and NO ACL (CLAUDE.md: the
// configuration is not a secret). A change that needs a store handle here
// has left this surface — say so in review rather than adding the field.
type configAPI struct {
    schema    func() *Schema      // App.State — snapshot ONCE per handler
    palette   *paletteService
    templater templating.Templater
    root      string              // project root for the apps/ scan
    // historyEnabled: a predicate, not the store.VersionService — this
    // surface must not be able to READ a version.
    historyEnabled func() bool
}

func newConfigAPI(schema func() *Schema, palette *paletteService,
    templater templating.Templater, root string, historyEnabled func() bool) (*configAPI, error)
```

Constructor validates required deps (CLAUDE.md: constructors reject nil). **Do
not** take `app *App` in the constructor — `newViewsHandler(app *App,…)`
(app.go:1333), `newAppearanceHandler`, `newQueryService`, `newExportHandler` did
that and it is a service-locator-shaped seam; this arc stops repeating it.

## Also in this PR

- Fix the stale prose at `worldneighbors.go:85` (cites `max-methods=104`;
the directive is 87 at app.go:172). Replace the restated number with a `[App]`
doc link — the commentlint `duplication` rule's remedy.
- Ratchet `//plimsoll:max-methods` at app.go:172 to the new exact count.
- Route ownership changes: verify `router_walk_test.go` still probes every
moved path (router.go:56).

## Invariants

- One `schema()` snapshot per handler (CLAUDE.md: capture state once).
`handleV1Config` (:1571) and `handleV1Schema` (:1289) already do this; keep it.
- No behaviour change; responses byte-identical for every principal.

## Done when

`go test -race ./internal/dataentry/...` green, `just plimsoll` with the lowered
directive, `just arch-lint`, `just comment-lint`, `just coverage-check`.
