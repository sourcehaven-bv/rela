---
id: REV-NVHQA3
type: review-checklist
title: 'Review: Gate the membership relation against ACL self-promotion'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` clean, incl. `-race` on acl/aclaudit/appbuild; 2026-08-19)
- [x] Lint clean (`just lint` 0 issues; `just arch-lint` OK; `just plimsoll` clean)
- [x] Coverage maintained (`just coverage-check`: package floors + total 76.6% >= 65% PASS)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (RR-5NIR95, RR-TVL38I — both fixed in the warn-test helper, verified with -race)
- [x] Self-reviewed the diff for unrelated changes (diff touches only the ticket's six files + two new tests)

**Review Responses:**

Code review: RR-5NIR95 (significant, addressed), RR-TVL38I (significant,
addressed), RR-EZ0P4S (minor, deferred: audit double-report follow-up),
RR-D3MEV0 (minor, addressed: docs wording), RR-6560A3 (minor, addressed:
shared test const), RR-8ZOICR (minor, deferred: asserted-roles audit gap,
flagged to architect).
Design review (earlier): RR-62ZH2M (minor, wont-fix), RR-S7A16Q (minor,
deferred to TKT-DN37J2 discussion).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (shared predicate semantics) — PASS: `TestPolicy_MembershipSelfPromotionOpen` + `TestRoleDef_IsPrivileged` (internal/acl/membership_gate_test.go).
- AC2 (A1 behaviour unchanged, aclaudit delegates) — PASS: existing aclaudit A1 tests green against the delegating code; manual `rela acl audit` shows the identical [high] A1 finding.
- AC3 (warning fires/quiet exactly per predicate) — PASS: appbuild_membership_warn_test.go (fires + 3 quiet shapes + no-acl.yaml) and manual CLI run (warning for ungated policy, silent when gated).
- AC4 (docs in both trees) — PASS: docs/acl-security.md + GUIDE-acl-security.md, incl. the coming TKT-DN37J2 refusal.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-3V181I

## Final Checks

- [ ] Commit message explains the why, not just what (pending: commit/PR timing is the architect's call)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->
