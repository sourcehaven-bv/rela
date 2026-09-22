---
id: IMPL-YJ63M1
type: implementation-checklist
title: 'Implementation: reverse_relation migration step: rewrite stored edges when a relation type''s direction is swapped'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units) — the step is
      exercised through `Runner.Run` over a real memstore and a parsed migration
      file, not by calling the step in isolation
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data — `relStore`,
      `reversedMeta`, `runReverse` helpers
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built a two-type project (ticket/feature) with two `blocks` edges, one carrying
body content, then swapped `from:`/`to:` in `schema.yaml`:

- `migrate status` reported ONE `relation_endpoints_swapped` delta, not two
  endpoint narrowings.
- `migrate gen` drafted a live `- reverse_relation: {type: blocks}` step (no
  "no declarative step can fix this" comment), and the draft parsed.
- `migrate data` dry-run: "would change 2 record(s)". `--apply`: "changed 2".
- Files re-keyed on disk (`TKT-1--blocks--FEAT-1.md` → `FEAT-1--blocks--TKT-1.md`)
  with the body content intact; the gate then reported in sync.
- `trace from FEAT-1` traverses in the new direction; `rela validate` clean.
- Re-running `migrate data --apply` is a no-op (the marker records the file as
  applied), which is what prevents the oscillation the design review found.

Refusal path, separately: a project holding both `TKT-1--blocks--FEAT-1` and
`FEAT-1--blocks--TKT-1` is refused naming both edges, and BOTH FILES SURVIVE.

A defect was found by this manual pass and fixed: the dry-run originally
reported "would change 2 record(s)" for that colliding project and only failed
on apply. The collision check now runs on the preview too
(`previewReversible`), so the message is identical on both. Pinned by
`TestReverseRelation_DryRunRefusesBothDirections`.

## Quality

- [x] Code follows project patterns (check similar code) — the capability
      follows `HeaderReader`/`Formatter` (optional, type-asserted, package-level
      dispatcher with a fallback); the step follows `renameRelationTypeStep`
- [x] Checked for DRY opportunities — the collision check exists twice on
      purpose (preview vs. fallback-write) and says why in a comment: they
      answer different questions, and a native backend has no read-only path to
      its own unique constraint
- [x] No security issues introduced — no new input surface; the step takes one
      relation type NAME validated against both embedded projections, and the
      raw-store trust boundary is the operator shell as for every other step
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

## Mutation verification

Three guards were disabled to confirm the tests actually pin them:

- `resolvedSubject` returning a bare type instead of `"rel:"+Type` → the
  enforcement seam breaks exactly as the design review predicted
  (`TestReverseRelation_FileMustCarryTheStep/correct_file_parses` fails).
- The self-edge skip in the fallback → `TestReverseRelation_SelfEdgeSurvives`
  fails (the edge is destroyed).
- pgstore's in-place `UPDATE` swapped for a DELETE+INSERT →
  `SwapRelationEndpointsKeepsTheLineage` fails, confirming the test pins the
  lineage claim rather than passing incidentally.
