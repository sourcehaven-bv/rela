---
id: REV-VAIHYL
type: review-checklist
title: 'Review: On postgres, a faced entity''s relations, clone and document routes are unreachable: the id segment is never parsed'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full suite with `-race`, exit 0.
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc
links across 15330 comments.
- [x] Coverage maintained (`just coverage-check`) — 79.9%, both thresholds
pass.

Also run: `just arch-lint` (OK), `just plimsoll` (clean).

**Comment findings.** None introduced. One lint finding WAS introduced and fixed
rather than suppressed: threading `entityRef` from the dispatcher pushed
`handleV1DynamicRoutes` to cognitive complexity 38 against a cap of 30. I
restructured to per-handler parsing instead of adding a `//nolint`.

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: reviewed against the same branch's
BUG-64MU2Q work, whose cranky-code-reviewer pass produced RR-5MLZCR — the
finding this change directly applies in `tailOfExistingEdge`.)
- [x] All critical review-responses addressed — none open.
- [x] All significant review-responses addressed — none open.
- [x] Self-reviewed the diff for unrelated changes.

**Self-review caught one real defect**, recorded because it is the kind a
reviewer would have flagged. My first `tailOfExistingEdge` discovered the tail
from the triple and ignored the caller's address. On a triple carrying edges at
two tails, a PATCH to `@published` would have written the draft edge — the exact
cross-face confusion this bug exists to fix, reintroduced by its own fix. Found
by re-reading my own doc comment, which claimed the ambiguous case was "out of
reach" when the change had just brought it into reach.

**Review Responses:** none new.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- *A faced address reaches the relations sub-tree and serves ITS OWN tail* —
PASS (`TestFacedAddress_RelationsSubTreeReachesItsOwnTail`). Verified to fail
without the fix: both faces served the union.
- *A PATCH addressed to one tail of a multi-tailed triple writes THAT edge* —
PASS (`TestFacedAddress_SingleRelationPatchHitsItsOwnTail`). Verified to fail
without rule 1: the `@published` PATCH wrote the draft edge.
- *A bare address still reaches the default tail* — PASS; the zero face
selects default-tail edges, and the whole existing suite exercises this.
- *The reserved-segment document case stays a 400* — PASS
(`TestExportDocument_RouteShapes`), after I regressed and fixed it.
- *No regression across the tree* — PASS, full `-race` suite.

**What is NOT verified, and why.** I could not complete live-server verification
on postgres: the current develop build hangs on startup against a migrated atlas
database, where the Sept-18 installed binary starts fine. I confirmed this is
NOT caused by this change by building the unmodified branch point, which hangs
identically. That is a separate develop regression, flagged but not diagnosed
here. Verification therefore rests on dispatcher-level tests that drive the real
routing, plus the earlier live-server measurement of the defect itself.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors. This is a bug.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
