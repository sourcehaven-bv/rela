---
id: DOCS-KGF71J
type: docs-checklist
title: 'Docs: export: per-list render override (lists.<id>.export_render)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on every new exported symbol — `lua.ListRows`, `ListQuery`,
`ListSortSpec`, `ListRenderContext`, `WithListDocumentMode`,
`Engine.ExecuteListDocument`
- [x] Load-bearing rationale documented at the seams: why rows are handed in
rather than re-queried, why `entry_id` is absent rather than empty, why
`resolveEffectiveList` omits the column predicate, why `runDocumentScript` stays
unexported and never takes a variadic option tail
- [x] Known residual documented where an implementer will find it — the
`Content` body pass-through is noted on both the `ListRows` seam and
`listOverrideRenderer`, pointing at the body-redaction TODO in
`internal/visibility/policyreader.go`

## Project Documentation

- [x] `docs/transforms.md` — removed the "Not yet supported (v1 limits)" bullet
that said list export always emits the column table; added a "Custom per-list
rendering" section mirroring the per-entity one (YAML + Lua example, the full
`rela.document` field table, and the two non-obvious contracts: render the rows
you are given, and rows are lazy so each walk re-materializes)
- [x] `docs/transforms.md` §Access control — extended the `export_render:`
bullet to cover `lists.<id>.export_render`, stating that a list override's rows
are server-supplied and already gated
- [x] `docs/lua-scripting.md` — added the list-render fields to the document-mode
reference, noting `rela.mode` stays `"document"` and `entry_id` is absent
- [x] `docs/data-entry.md` — added `export_render` to the List Fields table

## External Documentation

- [x] ~~API reference update~~ (N/A: no new endpoint or wire-format change —
the existing `GET /api/v1/{plural}/_export` gains no parameter; the override is
config-selected, never request-selected)
- [x] ~~Migration notes~~ (N/A: purely additive config key; a list with no
`export_render` keeps the built-in table byte-for-byte)
