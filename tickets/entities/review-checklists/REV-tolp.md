---
id: REV-tolp
status: in-progress
title: 'Review: Add SQL query tool to MCP server'
type: review-checklist
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained~~ (coverage-ignore on MCP handlers)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-pzwh, RR-k5mp, RR-pq3p, RR-7mu4 (all addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- PASS: sql_query tool accepts SQL and returns structured JSON
- PASS: Entity/relation tables exposed with correct pluralization
- PASS: SHOW TABLES and DESCRIBE work
- PASS: Error handling for invalid SQL

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending -->
