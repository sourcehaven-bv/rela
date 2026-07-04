---
id: REV-YTZ6LM
type: review-checklist
title: 'Review: ACL: make the membership relation (member-of) configurable via membership_relation: in acl.yaml'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full suite green, acl 88.5%; CI Test job green
- [x] Lint clean (`just lint`) — 0 issues; CI Lint + God-object lint green
- [x] Coverage maintained (`just coverage-check`) — floors + total (76.8%) PASS

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer) + `/crit` (human review, approved round 2)
- [x] All critical review-responses addressed (0 critical)
- [x] All significant review-responses addressed (RR-LFMR7S, RR-WG5NY1, RR-1659OA + design-review)
- [x] Self-reviewed the diff for unrelated changes (accidental declarative.go scratch edit reverted; git diff confirms only intended files)

**Review Responses:** RR-LFMR7S, RR-XKQZ1N, RR-WRFCW5 (design-review);
RR-WG5NY1, RR-1659OA, RR-HPZTJ1, RR-LAU07P (code-review). All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist (IMPL-ER8TOC)

**Acceptance Status:**
- AC1 group grant via configured relation → PASS (TestMembershipRelation_Configured_ConfersGroupRole)
- AC2 unset → default member-of → PASS (TestMembershipRelation_Default_WhenUnset)
- AC3 default-policy back-compat / wrong-relation not followed → PASS (existing suite + TestMembershipRelation_Configured_DoesNotFollowMemberOf)
- AC4 wrong relation → no role → PASS (negative assertion in AC3 test)
- AC5 hardening warning → PASS (TestPolicy_MembershipRelation_UngatedWarns / GatedNoWarn)
- AC6 docstrings + docs mention field + default → PASS (godoc + 3 doc files)
- Plus end-to-end blank-guard → PASS (TestMembershipRelation_BlankNeverQueriesMatchAll)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-7EL6ZA)
- [x] User-facing documentation updated (security.md, acl-overview.md, acl-security.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-7EL6ZA

## Final Checks

- [x] Commit message explains the why (single source of truth for the membership relation, blank-guard rationale)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (every job green except the "Rela Tickets" guard, which fails only because this very checklist was in-progress; checking these boxes + marking done clears it)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1060
