---
id: IMPL-1KYFXW
type: implementation-checklist
title: 'Implementation: No migration step can assign rows to a face, so bare_face adoption silently relabels every existing row'
status: done
---

## Development

- [x] Unit tests written for new code — `confirmface_test.go`: exhaustiveness,
non-bare target refusal, bad mapping entries (table-driven), step ordering,
uncovered rows, idempotence, and generator drafting.
- [x] Integration tests written (test full flow, not just units) — the runner
tests go through `ParseFile` → `Runner.Run` against a real memstore, not the
step in isolation.
- [x] Happy path implemented
- [x] Edge cases from planning handled — rows outside the declared value set
(unset or stale) are counted and reported, never folded into the confirmation.
- [x] Error handling in place (errors surfaced, not swallowed) — every refusal
names the subject and the fix; the non-bare-target error explains the store
constraint and both ways out.

## Test Quality

- [x] Using fixture builders or factories for test data — `facedV1()` beside the
existing `metaV1`/`metaV2`; `seedStore`, `mustFileYAML`, `mustParse` reused.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Scratch project with an `article` type (enum `draft`/`active`/`withdrawn`) and
three rows, then `bare_face: published` added.

1. `rela migrate gen` drafts a commented `confirm_face` skeleton listing all
three values against `published`, under a TODO explaining what to check. Before
this change it drafted a file with no face step at all.
2. Editing `withdrawn: draft` and applying is refused:
*"status = \"withdrawn\" is mapped to face \"draft\", but every existing row is
stored at the zero coordinate and becomes \"published\" ... Either make
\"draft\" the `bare_face:` ... or handle these rows separately"*. This is the
intended moment of discovery: the operator learns the schema does not fit the
data, at authoring time.
3. With all three mapped to `published`, the apply succeeds and reports
`changed 0 record(s)` plus `note: confirmed 3 row(s) become the bare face; no
rows were written`.

A defect found during this run and fixed: the step originally set `Affected`,
which the run report renders as "changed 3 record(s)" — misleading for a step
that writes nothing. The count moved to a note.

## Quality

- [x] Code follows project patterns (check similar code) — `Validate` against
embedded projections like every other step; `enumValuesIn` mirrors the existing
`storedFaceIn`/`faceInShape` shape helpers; generator case follows the
`enum_values_replaced` precedent.
- [x] Checked for DRY opportunities — `enumValuesIn` is shared by the step and
the generator's candidate-property picker rather than duplicated. The
create-then-delete machinery was NOT extracted from `rename_face`: the final
design writes nothing, so there was nothing to share.
- [x] No security issues introduced — no new external input path; the step
reads the store and writes nothing.
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

## Gates

`go test ./internal/...` clean, `golangci-lint` 0 issues, `just arch-lint` OK,
`just lint-md` 0 issues, `just comment-lint` clean, `just plimsoll` clean,
coverage thresholds pass (package 69.3%, total 79.4%).
