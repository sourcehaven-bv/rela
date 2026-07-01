---
id: REV-T7J4JV
type: review-checklist
title: 'Review: ACL: dedicated authorization-misconfiguration validator / audit insights (escalation foot-guns, dead assignments, un-gated membership)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — aclaudit + acl + cli + dependents green
- [x] Lint clean (`just lint`) — 0 issues (aclaudit, acl, cli)
- [x] Coverage maintained (`just coverage-check`) — aclaudit 90.3%; total 76.9%; floors PASS

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer) + `/crit` (human, round 1 addressed)
- [x] All critical review-responses addressed (0 critical)
- [x] All significant review-responses addressed (RR-EG5D3E — the A1 read-only false positive)
- [x] Self-reviewed the diff for unrelated changes (branch cleanly stacked on membership base; verified via `git diff feat/acl-configurable-membership-relation`)

**Review Responses:** design-review RR-LXI3NW, RR-UR0LJU, RR-O7H3GY, RR-TZ2S3G;
code-review RR-EG5D3E, RR-KUOAVH, RR-O50E4R, RR-4O11EZ. All addressed. Crit
round-1 comments (positive framing, --fail-on=any production guidance,
security.md clarification, e2e demo script) all addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (planning checklist PLAN-BFB9EJ)
- [x] Test evidence documented in implementation checklist (IMPL-NDR4NG)

**Acceptance Status:**
- AC1 un-gated membership + privileged assignment → A1 high, --fail-on non-zero → PASS (demo §2, unit)
- AC2 write-role on everyone → A3 critical → PASS (demo §3, unit)
- AC3 undeclared type → B1; membership undeclared → B2 → PASS (demo §4, unit)
- AC4 clean policy (incl. everyone: read: ["*"]) → zero findings, exit 0 → PASS (demo §1)
- AC5 --json → AnalysisResult envelope → PASS (demo §6, unit)
- AC6 membership warns gone from Validate; reproduced as aclaudit; Validate still
hard-errors structurally → PASS (unit; verified update⊆read still fires)
- AC7 arch-lint + plimsoll pass → PASS
- Bonus: --fail-on severity threshold (from user review) → PASS (demo §5, unit)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-B93LL3)
- [x] User-facing documentation updated (acl-security guide, security.md, demo script)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-B93LL3

## Final Checks

- [x] Commit messages explain the why (4 audit commits, each with rationale)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use (demo script + guide)

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** *pending — TKT-TS0J5K is stacked on PR #1060 (membership). Per the
agreed sequencing, wait for #1060 to merge to develop, then rebase this branch
onto develop and open the audit PR. REV stays in-progress until the PR exists +
CI passes (the "in-progress review checklist cannot be merged" CI guard is
correct here).*
