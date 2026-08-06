---
id: IMPL-136RHD
type: implementation-checklist
title: 'Implementation: Edit form hides properties the entity doesn''t have yet — newly added metamodel properties are unreachable on existing entities'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written — `DynamicForm.test.ts` (new file; none existed),
`affordances.test.ts` additions, backend `affordances_test.go` cases
- [x] Integration tests written — `api_v1_test.go` exercises the full GET/PATCH
wire contract end to end (handler → serializer → affordance service)
- [x] Feature implemented
- [x] Edge cases handled — see below

## Manual verification

Verified the tests actually catch the bug rather than merely passing: reverting
the edit-mode gate to its pre-fix form fails **4 of 5** render cases, including
the wizard path. The 5th ("hides a redacted property in a step") passed on the
old code for the wrong reason — it hid *everything* unset. That is exactly why
the render and hide assertions are paired; either alone can pass spuriously.

Also confirmed by DOM dump that a redacted property renders no input while an
unset one does, on the same response.

## Edge cases covered

- Unset property + `_fields: {}` (the bug) — renders
- Redacted property — does not render, value never on the wire
- Unset and redacted on the same entity — distinguished correctly
- Wizard path — both cases
- `_redacted: []` (permissive default) — hides nothing
- `_redacted` absent (list rows / non-per-entity shapes) — hides nothing
- List rows omit `_redacted` entirely (pinned)
- `_redacted` agrees exactly with what `stripHiddenProperties` removed (pinned)

## Quality

- [x] Follows project patterns — `_redacted` mirrors the existing
`Inaccessible` field and the `_fields`/`_relations` closed-world pointer
semantics; the predicate lives in the already-tested `utils/affordances`
- [x] No silent failures — the change's whole point is replacing a silent
inference with an explicit signal
- [x] `go test ./...` — pass
- [x] `npx vitest run` — 1440 pass (88 files)
- [x] `just lint` — 0 issues; `npm run lint` — 0 errors, no warnings in touched files
- [x] `just arch-lint` — OK
- [x] `just coverage-check` — pass (77.2%)
- [x] `just plimsoll` — pass
- [x] `npm run typecheck` — clean

## Scope corrections made during implementation

1. **The reported fix was based on a false premise.** `allFields` comes from the
data-entry form config, not the metamodel, so a metamodel-keyed fix would have
been both wider than needed and wrong for forms that deliberately omit a
property. Documented in BUGA-YBCFE1.

2. **The planned data-loss guard was unnecessary and was removed.** Planning
worried an untouched redacted field would submit as empty and clobber the stored
value. Edit mode has no bulk submit (`handleSubmit` returns early when
`isEdit`); all writes are per-property autosave, which fires only for fields the
user typed into. A guard against a path that does not exist would have been dead
code implying protection it did not provide. Two tests pin the reasoning so a
future bulk-submit path cannot silently reintroduce the risk.

3. **One pre-existing test needed correcting, not suppressing.**
`TestV1Affordance_PatchEcho_StripsHidden` substring-matched the whole body for
the field *name*, conflating name disclosure with value disclosure. Per
DEC-T0XIWQ the value assertion is the real invariant; the test now asserts on
the value and on `properties`, and additionally pins that `_redacted` names the
field.
