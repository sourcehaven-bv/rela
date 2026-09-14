---
id: DOCS-DSA2NZ
type: docs-checklist
title: 'Documentation: Create forms need a "Create & add another" button'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

`resetCreateForm()` carries the full rationale for each piece of state it
clears, because the enumeration is the part a future change will silently get
wrong. Several comments record *verified negatives* rather than implying a guard
that is real — `hiddenPolicy.releaseAll()` and the `pickerTypes` carry-over both
say plainly that they are belt-and-braces today and why they are kept anyway.
The `SubmitMode` type documents that the parameter must stay defaulted, since
`handleKeydown` and `defineExpose` both call bare.

One pre-existing comment was **corrected, not added**: `handleSubmit` claimed
`pendingCardChanges` is always empty in create mode. That was false (an incoming
`RelationPicker` populates it), and the false comment is why the leak in
RR-9EAUGR shipped.

Go: `KeepOnAddAnother` on both `FormField` and `FormRelation` documents the
opt-out default, why it is named for the button rather than "sticky", and that
it must serialize — unlike `FormRelation.Span`, whose `json:"-"` is the trap
next to it. The validator records why `mode: edit` is deliberately NOT rejected.

## Project Documentation

- [x] ~~README updated~~ (N/A: the README is a high-level feature list and a
docs index — verified it documents no individual form-config keys, so a
per-field option has no place in it)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern or convention — this follows
the existing `clear_when_hidden` config-key shape and the existing
`PendingButton` action pattern)
- [x] ~~Help text accurate~~ (N/A: no CLI change)

`docs/data-entry.md` gained a "Create & add another" section under create-form
behaviour. Edited at the **source**
(`docs-project/entities/guides/GUIDE-data-entry.md`) and regenerated via `just
docs` — `docs/` is generated, so editing it directly would have been reverted by
the next build. Verified idempotent.

Deliberately NOT placed beside `clear_when_hidden`: that entry lives under the
wizard-conditions section, and `keep_on_add_another` is not a condition key —
filing it there by analogy would misrepresent its scope.

The section states what carries over and what does not (including that the body
is always cleared), because an operator cannot predict the behaviour otherwise,
and a typo in the key is silently ignored (top-level-only strict key checking),
which makes the reference the only discoverability backstop.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo keeps no CHANGELOG; release notes are
generated from commits)
- [x] ~~API docs updated~~ (N/A: no HTTP API change — the new config key rides
the existing `Form` serialization; no new endpoint, request or response shape)
