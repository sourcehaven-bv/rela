---
id: IMPL-35TQGI
type: implementation-checklist
title: 'Implementation: export: per-list render override (lists.<id>.export_render)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] `internal/lua/listmode.go` — `ListRows` consumer-side interface (Len/At),
`ListQuery`, `ListSortSpec` (alias for `filter.SortSpec`), `ListRenderContext`
- [x] `internal/lua/runtime.go` — `WithListDocumentMode`;
`registerListDocumentFields` (a free function, not a `*Runtime` method);
`entry_id` made conditional so entity mode is byte-identical
- [x] `internal/script/list_document.go` — `ExecuteListDocument` typed seam +
`runDocumentScript` shared body, now also backing `ExecuteDocument`
- [x] `internal/dataentry/document.go` — `RenderListMarkdown`;
`documentScriptEngine` grew to 2 methods
- [x] `internal/dataentry/export_list.go` — `resolveEffectiveList`,
`listColumnsOf`, `listOverrideRenderer`, `entitySliceRows`, `buildListQuery`
- [x] `internal/dataentryconfig/` — `List.ExportRender` + shared
`validateExportRenderShape`
- [x] `internal/dataentry/app.go` — `checkExportRenderScripts` (lists **and**
views; the views half closes a pre-existing gap)

## Quality Checks

- [x] `go test ./...` — exit 0
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK; neither `lua` nor `script` gained a `dataentry` edge
- [x] `just plimsoll` — OK; `Runtime` stayed at its pinned max-methods=120 (the
first cut added a method and failed CI, so the helper became a free function
rather than bumping a pin TKT-ZF2DTV had just justified)
- [x] `just coverage-check` — floors PASS
- [x] `-race` on the export path — clean

## Verification Evidence

**End-to-end with a real Lua runtime** (not just the fake engine): a script on
disk rendering three real rows produced `entry_id is nil`, all three rows via
`rows()`, and "second walk saw 3" — the iterator restart contract holding in
practice.

**AC → test mapping**

| Acceptance criterion | Test |
|---|---|
| Override replaces the built-in table | `TestExport_List_RenderOverride` (asserts script output present **and** table header absent) |
| No override → table unchanged, script never runs | `TestExport_List_NoOverrideUsesBuiltin` |
| Override applies without an explicit `list=` | `TestExport_List_RenderOverride_EffectiveListFallback` |
| Columnless list still gets its override | `TestExport_List_RenderOverride_ColumnlessList` |
| Denied caller → 200, zero rows, no leak | `TestExport_List_RenderOverride_DeniedSeesNoRows` |
| Cap bookkeeping reaches the script | `TestExport_List_RenderOverride_TruncationReachesScript` |
| Filters/sort reach the script | `TestExport_List_RenderOverride_QueryContext` |
| Operator segment parsed, not swallowed | `TestExport_List_RenderOverride_FilterOperatorKey` |
| ConfigID cannot collide with the entity path | `TestExport_List_RenderOverride_ConfigIDDistinctFromEntityPath` |
| Lua surface: mode/list_id/count/total/query | `TestListDocumentMode_Surface` |
| `entry_id` absent in list mode | `TestListDocumentMode_EntryIDAbsent` |
| Entity mode unchanged (regression) | `TestDocumentMode_EntryIDStillPresent` |
| Iterator restarts per `rows()` call | `TestListDocumentMode_IteratorWalkableTwice` |
| Read-only context enforced | `TestListDocumentMode_QueryFrozen` |
| `total` clamped when under-reported | `TestListDocumentMode_TotalClamped` |
| Empty `q` is a string, not nil | `TestListDocumentMode_EmptyQIsEmptyString` |
| Edge cases (empty set, never-iterate, exhaust-then-rewalk, exact cap) | `TestListDocumentMode_EdgeCases` |
| Script seam: stdout, principal, error shaping, path validation | `TestExecuteListDocument_*` (4) |
| Config shape + boot-time existence | `TestValidateConfig_ListExportRender*` |
