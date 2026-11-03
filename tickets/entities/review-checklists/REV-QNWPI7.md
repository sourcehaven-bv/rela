---
id: REV-QNWPI7
type: review-checklist
title: 'Review: Access PR 1: golden wire-parity corpus, test ACL helpers, verifiedIdentity rename'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Evidence: `just ci` in the worktree on the rebased commit. Lint, comment-lint,
arch-lint and coverage pass. `cmd/rela-desktop`
TestChromeStyle_TargetsShippedClasses needs a built SPA and fails the same way
on develop. `imgproc` TestGIFBomb_RejectedWithoutDecode missed its 100ms bound
at load 100 and passed on rerun. `go test -race -cover -shuffle=on
./internal/dataentry/` passes in 262s, inside CI's 10 min package timeout.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** design review RR-YB4WLN, LNHFOK, PYCL3Q, JUE8Q5, FG5IF7,
Z49876, XO64B2, DMTLHS, M35RX4, FAZUH3, CHBUD2. Code review round 1 RR-VR1DYU,
XHFG11, 8BP7R4, TU68XN, DPRN31, HO56WW, CPK3QX, U274AN, 6C5477, D7BRXU;
RR-3UK73V wont-fix (nit). Security review RR-U2053C, V99XAQ, 1NY1H2, BB265N.
Round 3 RR-36PH04, A9RVZF, I0E3D6, UTB9MT, I49VIP, YJ0UY3, Y72F6V. All others
addressed.

Gaps the corpus surfaced are filed as bugs and recorded under bug-named cases:
BUG-2TPO2F, BUG-WJSYQT, BUG-C8DHGF, BUG-VUNO11, BUG-T6HNKS, BUG-E0CZZL,
BUG-E2CITD. BUG-HZ4FHZ and BUG-D8AM2A were fixed on develop by BUG-J01V0Q
(#1867) before this merged; their cases lost the bug prefix.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS. TestGoldenParity reads testdata/golden; `-update` rewrites it. A
deliberate one-field acl.yaml change failed 23 cases with a readable diff and
was reverted.
2. PASS. Each recording holds status, selected headers, normalized body and
store reads (calls by method, breadth, body loads).
3. PASS. Six identities under declarative header ACL, plus JWT, NopACL and
ReadOnlyACL variants, across all listed endpoints. A coverage test probes every
route and method; a non-405 method without a case fails.
4. PASS. `-count=20` and `-race` produce no diff.
5. PASS. Raw `app.acl =` / `app.fieldResolver =` remain only in
testhelpers_acl_test.go and rebindApp; the dataentry suite passes.
6. PASS. `jwtWired` appears in no `*.go` file.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor, no user-facing change)
- [x] ~~User-facing documentation updated~~ (N/A: internal refactor)
- [x] ~~Docs-checklist marked as done~~ (N/A: internal refactor)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
