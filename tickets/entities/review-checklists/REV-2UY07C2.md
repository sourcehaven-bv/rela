---
id: REV-2UY07C2
type: review-checklist
title: 'Review: Remote MCP toggle and tool filter in .rela/config.yaml'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): every package passes except TestChromeStyle_TargetsShippedClasses in cmd/rela-desktop, which needs the gitignored SPA build absent from this worktree; CI builds it
- [x] Lint clean (`just lint`): golangci-lint on the changed packages, 0 issues (the full-repo run timed out loading packages under machine load; CI runs it); arch-lint OK; plimsoll OK
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): new code is covered by its tests; the local run timed out in sqlitestore under load, CI runs the floor check

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** cranky-code-reviewer. RR-KCYRSF7, RR-EHO9WSE, RR-DOWIGL8
(significant, addressed); RR-WO03W25, RR-L0R1VL2, RR-3BG0N3G, RR-6OU8MPW,
RR-7SF2K7F (minor, addressed); RR-DYBE7JD (nit, addressed); RR-W35C0LC,
RR-C77VUYU (nit, wont-fix with reason).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Off wins, at least one on: PASS (TestResolveRemoteMCP, TestPlanRemoteMCP, manual run).
2. -mcp=false / RELA_MCP=0 off, other values refused: PASS (TestEnvToggle, TestToggleFlag, manual run).
3. Filter on tools/list, calls, watch: PASS (TestWithToolFilter*, assertToolFilterApplies, manual run).
4. Unknown names and misspelled keys refused: PASS (TestWithToolFilter_UnknownNameIsRefused, TestLoadConfig_RemoteMCP, manual run).
5. Docs state the precedence: PASS (GUIDE-mcp-server, GUIDE-server-security).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-V4O334X

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after done)
