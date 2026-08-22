---
id: TKT-PX5YL7
type: ticket
title: Document render scripts hold the full write surface on an HTTP GET
kind: refactor
priority: low
effort: m
status: backlog
---

## Problem

A document render is a GET that renders markdown. Its Lua script nonetheless has
the **full write surface**: `rela.create_entity`, `update_entity`,
`delete_entity`, `create_relation`, `delete_relation`, `write_file`.

`documentService` renders via `App.luaWriteDeps()`, and `runDocumentScript`
(`list_document.go:85`) calls `NewWriterRuntime` -> `lua.NewWriter` ->
`newRuntime(allowWrites=true)`. `registerWriteBindings` has no `isDocument`
guard — `isDocument` gates only `print`, `rela.mode` / `rela.document.*`, and an
`rela.output()` warning.

This is **not a privilege escalation**: those writes go through
`entitymanager.Manager`, so they are ACL-checked against the request principal
and audited like any other write. Nothing bypasses the ACL.

It is a **shape problem**:

- A GET that mutates is not idempotent. A retry, refresh, prefetch or
double-click re-runs the writes.
- It blocks caching. RR-1DV8RY notes elevated renders are principal-independent
and therefore uniquely cacheable; a render with side effects cannot be cached or
deduplicated safely. TKT-OGR566 and RR-P4E9GL both assume renders are pure.
- It contradicts the capability-bundle rule in the root CLAUDE.md — "split by
read vs. write so read-only code can't accidentally mutate state."

It also undercuts a claim TKT-Y3JVFK now makes carefully. That ticket had to
weaken five comments from "a render cannot mutate" to "cannot write past the
ACL" (RR-DOCWRT, critical) precisely because of this gap. Closing it would let
the stronger statement be true.

## The trap: `bypass_acl` is registered inside the `allowWrites` branch

**Read this before starting.** `registerBindings` (`internal/lua/runtime.go`)
registers `rela.bypass_acl` INSIDE `if allowWrites { ... }`:

```go
if allowWrites {
    r.registerWriteBindings(rela)
    if r.deps.ElevatedManager != nil || r.deps.ElevatedReader != nil {
        r.L.SetField(rela, "bypass_acl", ...)
    }
}
```

So any change that makes a document render non-writing ALSO removes
`rela.bypass_acl`, silently killing the elevated documents shipped in
TKT-Y3JVFK (#1366). There is no compile error.

Compounding it: `lua.NewReader(d ReadDeps, ...)` takes only `ReadDeps`, while
`ElevatedReader` lives on `WriteDeps` — so a reader runtime **structurally
cannot carry** the elevated handle at all.

Verified by simulation: forcing `allowWrites = false` when `isDocument` makes
`TestElevatedRender_ReadsHiddenEntityAndAudits` fail with "the elevated reader
did not arrive". That test is the safety net; do not weaken it.

## Proposal: two ordered steps

**Step 1 — decouple `bypass_acl` registration from `allowWrites`.**
Move the registration out of the writes branch, keyed only on the elevated
handles, which is already its real condition. Worth doing on its own merits:
registration currently depends on a flag it has no logical relationship to.
Small, safe, no behaviour change (today `allowWrites` is true wherever an
elevated handle exists).

**Step 2 — suppress the write bindings for document mode.**
Only after step 1. Two shapes:

1. `lua.NewReader` / a `ReadDeps` render path. Cleanest in principle, but
requires elevation to reach a reader runtime — i.e. `ElevatedReader` must move
to `ReadDeps`, or `NewReader` must grow a variant. Larger than it looks.
2. Skip `registerWriteBindings` when `isDocument`. Smaller, and keeps the
`WriteDeps` shape elevation depends on. The guarantee is comparable: the
bindings genuinely do not exist on the `rela` table either way, so a script
calling `rela.create_entity` gets "attempt to call a nil value".

Option 2 is now preferred, reversing this ticket's original preference. The
"structural vs. code path" argument for option 1 is weaker than it sounded once
`NewReader`'s inability to carry elevation is accounted for.

## Scope

`runDocumentScript` is shared by THREE entry points, so any change hits all of
them — verify each:

- `Engine.ExecuteDocument` (entity-anchored documents)
- `Engine.ExecuteStandaloneDocument` (standalone documents, TKT-M1AX6P)
- `Engine.ExecuteListDocument` (list renders, which feed `export_render`)

The list/export path is the one most likely to have a legitimate reason to
write, and is the least covered by this ticket's reasoning.

Partly resolved during design review (RR-PXLIST): the export endpoints ARE
GETs (`export_list.go:33`, `export.go:103`/`:126` all reject non-GET), so the
idempotence argument carries there unchanged. What remains open is whether
`export_render` has a legitimate reason to write — stamping "last exported at",
or recording an export-audit entity, is the most plausible legitimate render
write in the codebase, and is exactly what this change would break. Decide it
before implementation, not while making the diff uniform.

## Design review findings

- **RR-PXWF (significant)** — `write_file` is categorically different from the
five graph mutations: it is filesystem-only, confined to `output/`, path
validated, produces no audit row, and does not make a render non-idempotent
*with respect to the graph*. The caching rationale does not transfer. Scope it
out with a reason, or argue its removal on its own terms — do not let an
implementer delete all six because they share a function.
- **RR-PXLIST (significant)** — see above.
- **RR-PXSTEP1 (minor)** — step 1 is safe only because `allowWrites` is
currently true wherever an elevated handle exists, a property step 2
deliberately removes. Say so, and pin `bypass_acl` registration with a test
against a runtime built WITHOUT writes, so step 1 cannot be landed alone months
later with the reasoning lost.
- **RR-PXAC (minor)** — no acceptance criteria, for a change whose main risk is
silent breakage. Name the canary that must stay green
(`TestElevatedRender_ReadsHiddenEntityAndAudits`), the positive assertion to add
(`rela.create_entity` absent in a document runtime), and coverage for all three
entry points.

## Compatibility

No in-tree document script writes. The repo's only document configs are in
`prototypes/data-entry/project/data-entry.yaml`, and both their scripts
(`docs/category_report.lua`, `docs/status_review.lua`) make zero write calls.
The three scripts that DO write (`tickets/scripts/stale-review.lua`,
`examples/idp-sync.lua`, `scripts/generate-docs.lua`) are an automation, a sync
job and a docs generator — none are renderers.

This is still a **behaviour change for downstream deployments**: a document
script that writes today would break. Needs a deliberate call on whether that is
a bug fix (writes on GET were never intended — `ExecuteDocument`'s godoc
describes the script's output as the rendered markdown and says nothing about
mutation) or a breaking change warranting a deprecation path.

`rela.write_file` deserves separate thought: a render writing a side file is the
most plausible legitimate use, and it is not a graph mutation.

## Non-goals

- Changing the ACL treatment of document reads (TKT-Y3JVFK, shipped).
- Removing write bindings from non-document script surfaces.
- Implementing render caching (RR-1DV8RY / TKT-OGR566) — this unblocks it.
