---
id: REV-8B6S1PJ
type: review-checklist
title: 'Review: Narrow the list_entities Go path for filters that do not lower'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): ran every changed package with GOENV=off on the default tag, memorybackend, sqlite and postgres (predicate, queryplan, store, memstore, fsstore, sqlitestore, pgstore, affordances, visibility, appbuild, mcp, archguard, cli, dataentrywire)
- [x] Lint clean (`just lint`): golangci-lint on the changed packages, 0 issues; arch-lint clean; plimsoll clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: floors are per package, every changed package gained tests, and CI enforces the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-XW9LB6B, RR-1752VXK, RR-3P8YW21 (significant, addressed); RR-9L7TAS0, RR-6P2JE80, RR-XIKAH0L, RR-VLVHI2E, RR-K04G913 (addressed); RR-S5DXC8K, RR-JXPWGOF, RR-PCZ0WAQ (deferred with reason); RR-VZSZO2V (wont-fix with reason). The security review found no critical or significant issues.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- filterInGo removed and its allowlist entry gone: PASS (archguard test passes).
- Unsupported filters refused with the construct named: PASS (TestHandleListEntities_UnloweredFilterIsRefused, TestLowerConditionExplained_NamesTheConstruct, manual stdio run).
- Exact total counted in the store: PASS (MCP differential checks CountMatched and no whole-type read).
- No leak of hidden visible: values: PASS (hidden-field differential on memstore, sqlite, postgres; security review).
- Tool description and guide list supported shapes: PASS (tools_list golden, docs/mcp-server.md).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-U81IT26

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: the PR is opened after the ticket is done
