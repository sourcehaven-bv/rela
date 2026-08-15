---
id: TKT-PX5YL7
type: ticket
title: Document render scripts hold the full write surface on an HTTP GET
kind: refactor
priority: low
effort: s
status: backlog
---

## Problem

A document render is a GET that renders markdown. Its Lua script nonetheless has
the **full write surface**: `rela.create_entity`, `update_entity`,
`delete_entity`, `create_relation`, `delete_relation`, `write_file`.

`documentService` renders via `App.luaWriteDeps()` (`app.go:321`), which sets
`EntityManager`, and `runDocumentScript` (`list_document.go:85`) calls
`NewWriterRuntime` -> `lua.NewWriter` -> `newRuntime(allowWrites=true)`.
`registerWriteBindings` (`runtime.go:787`) has no `isDocument` guard —
`isDocument` gates only `print`, `rela.mode` / `rela.document.*`, and an
`rela.output()` warning.

This is **not a privilege escalation**: those writes go through
`entitymanager.Manager`, so they are ACL-checked against the request principal
and audited like any other write. Nothing bypasses the ACL today.

It is a **shape problem**:

- A GET that mutates is not idempotent. Anything that legitimately re-issues a
render — a retry, a refresh, a prefetch, a double-click — re-runs the writes.
- It breaks caching. RR-1DV8RY (on TKT-Y3JVFK) notes elevated renders are
principal-independent and therefore uniquely cacheable; a render with side
effects cannot be cached or deduplicated safely. TKT-OGR566 (bound concurrent
Lua document renders) and RR-P4E9GL (no dedup / no concurrency cap) both assume
renders are pure, and are harder to reason about while they are not.
- It contradicts the capability-bundle rule in CLAUDE.md — "split by read vs.
write so read-only code can't accidentally mutate state". A renderer is
read-only code that currently holds a write capability.
- `lua.NewReader` (`runtime.go:307`) already exists, so the read-only posture is
a first-class thing document renders simply do not use.

## Proposal

Decide whether document mode should drop write bindings, and if so how:

1. **Build the render runtime from `ReadDeps` via `lua.NewReader`.** Cleanest —
the capability is absent, not merely unused. Requires `runDocumentScript` and
`documentService` to stop threading `WriteDeps`.
2. **Keep `NewWriter` but skip `registerWriteBindings` when `isDocument`.**
Smaller diff; the handle still exists on the deps struct, so the guarantee is a
code path rather than a missing capability.

Option 1 is preferred on the same reasoning as TKT-Y3JVFK's read-only elevated
handle: make the restriction structural. A script calling `rela.create_entity`
in a document then fails with "attempt to call a nil value" rather than silently
mutating on a GET.

## Compatibility

No in-tree document script writes: the repo has no `documents:` configured in
`tickets/data-entry.yaml`, and the three scripts that do call write bindings
(`tickets/scripts/stale-review.lua`, `examples/idp-sync.lua`,
`scripts/generate-docs.lua`) are an automation, a sync job and a docs generator
— none are document renderers.

This is still a **behaviour change for downstream deployments**: a document
script that writes today would break. Needs a deliberate call on whether that is
a bug fix (writes on GET were never intended) or a breaking change warranting a
deprecation path. The godoc on `ExecuteDocument` describes the script's output
as the rendered markdown and says nothing about mutation, which supports reading
it as a bug.

Note `export_render` and list-document renders share `runDocumentScript`, so any
change here applies to them too — verify none of those paths rely on writes.

## Non-goals

- Changing the ACL treatment of document reads (that is TKT-Y3JVFK).
- Removing `write_file` from non-document script surfaces.
