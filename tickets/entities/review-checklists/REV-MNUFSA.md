---
id: REV-MNUFSA
type: review-checklist
title: 'Review: ACL-bypass automation scripts: rela.bypass_acl(closure) with a scoped, invalidated-after elevated write handle'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->

---
**Review note:** security-sensitive (ACL bypass). Design pressure-tested by the
go-architect agent, which caught a fatal leak in the original ctx-marker
approach (elevation would propagate into the nested cascade); reworked to an
object-capability handle (Manager.Elevated() / gated() at cascade dispatch).
Tests pin: bypass+audit-marker with real principal & no denied-write row;
elevation not leaking onto the gated Manager; the nested-cascade LEAK test
(downstream write still denied); the escaped-handle invalidation; rela.bypass_acl
absent without allow_acl_bypass. arch-lint + golangci-lint clean. Stacked on #994.
Self-reviewed.
