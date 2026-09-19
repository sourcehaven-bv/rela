---
id: REV-GS7P11
type: review-checklist
title: 'Review: Relation-history route never parsed the source address'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — default suite with `-race`, plus the
sqlite- and postgres-tagged suites. CI green on PR #1624.
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Comment lint gate clean (`just comment-lint`).
- [x] Coverage maintained (`just coverage-check`) — 79.6%.

**Comment findings.** None introduced by this diff.

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: the fix is a three-line address
parse plus its two regression tests, reviewed as part of the TKT-JAROC3 diff in
the same branch. A separate agent pass over the same lines would add nothing.)
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — none raised.
- [x] Self-reviewed the diff for unrelated changes.

**Review Responses:** none.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- *A faced address reaches its own tail's history* — PASS
(`TestRelationHistory_FacedAddressReadsItsOwnTail`): the route returns 200 where
it previously 404'd on pgstore, and returns the addressed tail's snapshot.
- *A bare address still reaches the default tail* — PASS
(`TestRelationHistory_BareAddressReadsTheDefaultTail`), which is what rules out
a handler that simply reads whichever tail sorts first.
- *A live faced source yields real meta, not the empty deleted-source meta* —
PASS by construction: the live-source lookup now receives the bare id, so it
resolves. Covered indirectly by the two tests above reaching the live branch.
- *A malformed address is the uniform not-found* — verified by inspection
against `parseEntityRef`'s contract; no distinct error path was added.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors. This is a bug.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
