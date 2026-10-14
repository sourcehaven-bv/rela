---
id: REV-RQPDVA
type: review-checklist
title: 'Review: Gaining a role can narrow field visibility and writability'
started: "2026-10-13"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Evidence: `just ci` on the rebased commit. Lint (all build tags) reports 0
issues; comment-lint, arch-lint and plimsoll pass. Every Go package passes under
`-race -shuffle=on` except `cmd/rela-desktop`
TestChromeStyle_TargetsShippedClasses, which needs a built SPA and fails the
same way on develop. Coverage is enforced by GitHub CI on the PR.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** design review and code review, 21 in total. Significant:
RR-9IULPS, LYV3VZ, M9FN24, MI8UCQ, OHFY9I, OQII5Y addressed; RR-MQ0ZK1 (relation
create/remove/meta writes keep the old fold) deferred to BUG-826XOX. Minor and
nit: RR-9T6J5B, D3BPII, DCED2P, LZLDVD, PZ5ADD, TR2VO3, XVC7I4, YV8D99, Z9128F,
5AUG0F, 8ENIVL, XU9H33 addressed; RR-Y1KTOI and RR-PUXMVI wont-fix. Security
review found no critical or significant issue.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS. A block-less role contributes every field or option when it holds
the matching verb on the row's type and face (roleunion_test.go, table per
dimension; create_face_union_test.go for faced create).
2. PASS. Monotonicity (MEAS-ROLE-UNION-MONOTONIC): TestRoleUnion_Monotonic
over random role subsets with conferred roles, everyone, `*` grants, relation
meta and a client ceiling, against a hand-written oracle. With the field union
disabled 3,680 checks fail.
3. PASS. The viewer, which reads tickets only through a conferred role, stays
restricted (golden corpus viewer recordings unchanged).
4. PASS. The client ceiling still only removes (ceiling variant of the
property test).
5. PASS. History keeps globals only and never shows more than a live read
(TestRoleUnion_History).
6. PASS. Golden corpus: 9 recordings change, all the editor (and an asserted
editor) now seeing budget and note on watched rows.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix; the union rule is documented in docs/acl-security.md and docs/acl-overview.md in this change)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; ACL guides updated anyway)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
