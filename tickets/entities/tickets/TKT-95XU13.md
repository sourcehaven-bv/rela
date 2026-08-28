---
id: TKT-95XU13
type: ticket
title: 'export: per-list render override (lists.<id>.export_render) — script receives handler-resolved rows lazily'
kind: enhancement
priority: medium
effort: m
status: done
---

## Summary

Closes the v1 limit recorded in `docs/transforms.md`: *"A render override for
LIST export — `export_render` applies to entity export only; list export always
emits the column table."* Anticipated by FEAT-5IUVGX ("List export: table in v1,
Lua override for fancier").

```yaml
lists:
  tickets:
    entity_type: ticket
    columns: [...]
    export_render: docs/ticket_report.lua
```

## Design decisions

1. **Keyed on the list, not the entity type.** Two lists of the same type can
export differently, and the list already owns the columns/filters/sort the
export derives from.

2. **The script renders handler-resolved rows; it never re-queries.** The
handler has already resolved the exact ACL-scoped, filtered, sorted, capped set.
Note the ACL argument does *not* apply here — TKT-ZF2DTV landed, so a script's
own reads are ACL-bound anyway. The real reasons are narrower: a re-query would
ignore the user's active filters and escape `listExportCap`, so the export would
stop matching the view it came from.

3. **The resolved query is read-only context** (`q`, `filters`, `sort`), frozen
via `freezeTable`, so a script can title and annotate an export. No re-query
seam; parameter rewriting would be additive and needs its own decision.

4. **Rows are lazy, not a prebuilt array.** `rows()` mints a fresh cursor per
call and materializes one `EntityToTable` at a time. A prebuilt array would
allocate a table + properties sub-table + 2 closures per row for the whole set
before the script's first line, and would have forced a second, separately-tuned
cap. Walking twice re-materializes — CPU, not memory.

5. **List mode rides on document mode** (`isDocument = true`), rather than a
third mode: a list render wants document mode's stdout capture and `rela.output`
guard verbatim, and a parallel mode would need OR-ing into every such guard
forever. `rela.mode` stays `"document"`; scripts discriminate on
`rela.document.list_id`.

6. **`rela.document.entry_id` is absent (Lua nil) in list mode**, not
empty-string — `""` is truthy in Lua, so the idiomatic guard would pass and then
raise, and `entry_id or default` would yield `""`.

7. **A denied caller gets 200 with an empty row set, not a 404** — matching the
non-override list export, since an empty list is indistinguishable from no
access.

## Load-bearing implementation notes

- **`resolveEffectiveList` deliberately drops the `len(Columns) > 0` predicate**
the original `exportListColumns` had. That is a *columns* concern; folding it
into list identity would make a list with `export_render` and no `columns`
silently not get its override. Behavior change: naming a columnless list now
falls back to `[id,title]` instead of borrowing the type-default list's columns.
- **Override selection happens AFTER** `scopedEntities` → `visReader.Filter` →
cap. That ordering is the guarantee that a script can only ever be handed gated,
capped rows.
- **`registerListDocumentFields` is a free function**, not a `*Runtime` method:
it needs nothing from the Runtime. (It also keeps `Runtime` off its plimsoll
line at max-methods=120 — a consequence, not the reason.)
- **`runDocumentScript`** backs both `ExecuteDocument` and
`ExecuteListDocument`. It stays unexported and takes the mode as a single
`lua.Option` parameter, never a variadic tail — that is what keeps the typed
seam structural rather than conventional.

## Riding along

- **Fixed a pre-existing gap**: `views.<type>.export_render` was never
existence-checked at boot. Now both override kinds are, via
`checkExportRenderScripts`, and both share `validateExportRenderShape`.

## Known residual

Rows reach the script through `EntityToTable`, which includes the entity
**body**. Property-level `visible:` redaction is applied, but
`visibility.Redact` does not yet redact `Content` (the body-redaction TODO in
`internal/visibility/policyreader.go`). This matches the entity export path,
which already exposes bodies — but the list export surface is newly widened, and
both the seam godoc and `listOverrideRenderer` now point at that TODO so whoever
implements body redaction finds this path.
