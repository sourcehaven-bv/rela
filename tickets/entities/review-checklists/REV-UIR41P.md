---
id: REV-UIR41P
type: review-checklist
title: 'Review: Remote MCP part 1 — go-sdk v1.7.0 migration + ACL-gated read seam'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full suite green on the merge commit
- [x] Lint clean (`just lint`) — `golangci-lint` 0 issues, `just arch-lint` OK, `just plimsoll` OK
- [x] Coverage maintained (`just coverage-check`) — package floors held

Merged as #1336 (squashed into #1338 as `6276f3e2`).

## Code Review

- [x] Run `/code-review` command — cranky-code-reviewer run during the arc
- [x] All critical review-responses addressed — none open
- [x] All significant review-responses addressed — RR-B7ZHYO, RR-CFFL52, RR-FTJUUE, RR-OMB6ID and siblings all `addressed`; the three that only bite over a network transport moved to TKT-BDG8U9 rather than being closed
- [x] Self-reviewed the diff for unrelated changes

## Verification

- [x] Acceptance criteria met — SDK migrated to v1.7.0 with byte-identical `tools/list` / `tools/call` output, and every MCP read routed through one ACL-gated seam
- [x] Manual testing performed — goldens frozen BEFORE the migration, then mutation-tested; one vacuous ACL test was found this way (it passed with the fix reverted) and replaced with `TestACL_BuildStoreRelations_WithholdsUnreadableEdge`
- [x] No regressions introduced — `rela mcp` behaviour unchanged under its NopACL wiring
- [x] Documentation updated — see DOCS-UIR41P

## Retrospective note

This checklist was written after the fact: #1336 merged without it because the
ticket gate never ran on that PR (it targeted a feature branch — see
BUG-CI7XKP). Recording it now rather than back-dating a claim that it was
followed at the time.
