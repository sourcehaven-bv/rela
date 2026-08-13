---
id: PLAN-SU7EFD
type: planning-checklist
title: 'Planning: CalDAV prep: VTODO renderer + completion fields in internal/calfeed'
status: done
---

<!-- @managed: claude-workflow v1 -->

Retrospective: this ticket was small (effort `s`), fully specified by
**RES-1Y2EB5**, and planned interactively rather than through this checklist
before implementation. Recorded here so the decisions are durable.

## Understanding

- [x] Requirements clear from the ticket + research
- [x] Existing code reviewed (`internal/calfeed/{calfeed,ical,json}.go` and
their tests)
- [x] Constraints identified

The package is a pure model→bytes leaf: no store, no metamodel, no vendor. It
was deliberately built event-granular in Phase 1 *for this ticket* —
`RenderEvent` / `RenderCollection` are already split so a CalDAV per-resource
GET can reuse the per-entry renderer, and `ETag` / `CollectionTag` already
exist.

## Research

- [x] ~~`/research` run~~ (N/A: covered by RES-1Y2EB5, which includes wire
captures from a live Apple Reminders sync)

## Approach

- [x] Design decided and recorded

**The one open decision was the model shape**, put to the user explicitly:

1. **Separate `Todo` type + `Component` discriminator on `Feed`** — CHOSEN
2. Extend `Event` with todo-only fields and branch in `RenderEvent`
3. A shared interface over both

Rationale for (1): a VTODO's time anchor is `DUE` not `DTSTART`, and the
completion trio has no VEVENT meaning, so a shared struct would dangle todo-only
fields on every calendar event. More importantly it makes "a collection is
VEVENT-only or VTODO-only" — which **Apple requires**, verified twice in the
live test — structural rather than a runtime check. Cost is some duplication
between `ETag`/`TodoETag` and the two `CollectionTag` branches, accepted.

Rejected (2) because it makes the invariant a convention. Rejected (3) because
`calfeed` is deliberately plain structs and pure functions with no interface
indirection anywhere.

## Security

- [x] Security implications considered

The only surface is **content injection**: a CRLF in a summary or description
could forge a property line (e.g. a fake `STATUS:COMPLETED`). The existing
`escapeText` / `stripLineBreaks` machinery handles it and is reused verbatim;
`TestRenderTodo_NoLineBreakInjection` pins it, mirroring the VEVENT guard.

No ACL surface — this package never touches the store. No secrets. No external
input parsed (render-only).

## Test plan

- [x] Test approach decided

Table-driven subtests in the existing `ical_test.go` idiom, reusing its
`logicalLines` / `unfold` / `containsLine` harness so CRLF and folding are
checked by construction.

Plus a distinct class: **fixture tests against byte-exact captures from Apple
Reminders**, committed to `testdata/`. These pin observed client behaviour (the
completion trio; `DTSTART` invented to mirror `DUE`; bare-UUID UIDs on
client-created todos) as executable documentation, so a future change that
breaks an assumption fails loudly rather than in the field.

Note: fixtures need `.gitattributes` (`*.ics -text`) or git's eol normalisation
rewrites CRLF→LF and silently invalidates them.

## Risk assessment

- [x] Risks identified

- **Low blast radius.** Additive only: `Component`'s zero value is
`ComponentEvent`, so every existing `Feed` renders exactly as before. Verified
by the untouched Phase-1 tests plus `internal/dataentry` /
`internal/dataentryconfig`.
- **Main risk is a half-set completion state** — RFC 4791 §7.8.9 filters on
`COMPLETED` while UIs read `STATUS`, so setting one without the other reads as
done in one client and pending in another. Mitigated by `Todo.Complete`, which
sets all three together.
- **Interop risk deferred**, not avoided: real per-client conformance
(Reminders, Thunderbird, eM Client, Cfait) belongs to TKT-MF1CWZ once there is a
protocol surface to test against.
