---
id: REV-7U53XDX
type: review-checklist
title: 'Review: Conflict list and detail endpoints skip the read gate'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-I1I3Y8Z (critical, deferred to BUG-HTSW37Q),
RR-RQ0M3G6, RR-U6VT8K3, RR-2PDNIPS, RR-FICTL99 (significant, addressed),
RR-2OKF1P7, RR-SUZWM52, RR-NEUFOGD (minor, addressed), RR-XCAZEMK (minor,
wont-fix), RR-QINIFW9 (minor, deferred), RR-NF1EIMI, RR-GPDMROX, RR-OZC6RNS
(nit, addressed). RR-U6VT8K3, RR-NF1EIMI and RR-GPDMROX are linked to
BUG-0E6S7Q, the rest to BUG-Q3Z15V. RR-9LYVP2G (significant, addressed)
came from the security review of the PR.

The critical finding fails closed with a 500 and discloses nothing. It is a
store-level behaviour shared with the type's list routes, so it is tracked in
BUG-HTSW37Q.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: the list leaves out files naming a hidden row or edge
(TestV1Conflicts_ReadGate).
- PASS: the detail and the resolve answer the uniform 404 for a hidden file,
byte-equal to a missing file (TestV1Conflicts_HiddenAnswersMatchMissing,
TestV1ConflictResolve_HiddenRowIs404).
- PASS: the detail serves no value of a `visible:`-hidden property
(TestV1ConflictDetail_RedactsHiddenProperties).
- PASS: resolve authorizes the file's own row through
entitymanager.AuthorizeUpdate and refuses a resolved identity that differs from
the file (TestV1ConflictResolve_IDMustMatchFile,
TestV1ConflictResolve_FileIdentity, TestV1ConflictResolveACLDenied).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

<!-- The PR URL and CI status are deliberately not tracked here (TKT-UFV01M). -->
