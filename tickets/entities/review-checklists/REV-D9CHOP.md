---
id: REV-D9CHOP
type: review-checklist
title: 'Review: Expose request principal to Lua runtime (rela.principal) for write-path authorship'
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
**Review note:** read-only `rela.principal` via a frozen proxy table
(__newindex raises, __metatable locked). Reframed the PLAN-XKMJ AC13 spoofing
test: kept the three rewrite-vector probes (rela.audit / with_principal /
with_triggered_by must stay absent), replaced the "rela.principal must not
exist" probe with positive read-only + reflects-ctx + falls-back-to-unknown
tests. Write attribution still derives from callerCtx() (luaCreateRelation:
runtime.go), structurally independent of the table — reading identity is not a
forge path. Docs updated (GUIDE + generated). Self-reviewed.
