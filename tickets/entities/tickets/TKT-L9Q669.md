---
id: TKT-L9Q669
type: ticket
title: 'export: route entity/list export + export_render through visibility.Reader; thread request principal into ExecuteDocument (closes #1188 IB finding)'
kind: enhancement
priority: high
effort: m
status: backlog
---

## Summary

PR 2 of the FEAT-PPH1EU arc (DEC-ZBI39P). Fixes the CISO-blocked IB-review
finding on PR #1188: export paths read properties straight off the fetched
entity, bypassing field-level `visible:` redaction that every other data-entry
response applies.

Leaking sites (from the IB review + verified on the branch):
- `internal/dataentry/export.go` — `exportRenderer` hands the raw entity to `transform.EntityRenderer`; `propertyRows()` ranges the full `Properties` map; `DisplayTitle` re-leaks a hidden display property through the H1/filename.
- `internal/dataentry/export_list.go` — `columnCell` reads `e.Properties[c.Property]` directly; property columns in list export never consult `hiddenProperties`.
- The `export_render:` Lua override — `script.ExecuteDocument` takes no ctx and runs Lua reads on `context.Background()` with **no principal** (RES-PSZZKU finding), so the operator script reads raw entities regardless of the caller.

## Scope

- Route entity export + list export reads through `visibility.Reader` (TKT-7I07IX) instead of raw property access — redaction happens at the seam, `transform.EntityRenderer` stays ACL-free (it renders what it is given).
- Title fallback: exported H1 and download filename must not carry a hidden display property (Reader's copy already applies this — verify through the export path).
- Thread the request ctx/principal into `ExecuteDocument` so the `export_render:` Lua path runs under the caller's principal; until PR 3 lands the Lua reads themselves stay raw, so ALSO pass the redacted entity boundary where feasible, or document the residual explicitly in the PR + `docs/transforms.md` §Security if the override path can only be fully closed by PR 3.
- Negative tests: a principal with a field-visibility policy exports an entity/list/PDF and the hidden field's value appears nowhere in the output bytes; hidden display property → ID title; list relation columns already gated (regression-pin).
- Update `docs/transforms.md` §Security notes.
- Resolve the open review-response on #1188 (CHANGES_REQUESTED by CISO): field-level ACL redaction on both export paths.

## Non-goals

Lua read bindings (PR 3). MCP export. CLI `rela render` (operator trust boundary
— has the raw files anyway).
