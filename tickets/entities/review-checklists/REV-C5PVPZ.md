---
id: REV-C5PVPZ
type: review-checklist
title: 'Review: t.Parallel wave + -shuffle=on'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Per-package `-race -count=2 -shuffle=on` green (lua + acl at -count=4 after fixes)
- [x] `golangci-lint run` — 0 issues across all 8 wave packages (tparallel satisfied)
- [x] Full `just ci` green (includes the new -shuffle=on flags)

## Code Review

- [x] `/code-review` run (cranky-code-reviewer on the wave diff)
- [x] Findings recorded: RR-9DZXKT (critical — global-slog race, reproduced, addressed), RR-J730O4 (significant — latent variant, addressed), RR-02PMQL (minor — class-rule documentation, addressed)
- [x] All critical/significant findings addressed
- [x] Reviewer independently confirmed the "safe to share" classifications (validator creates fresh Lua state per check; resolvers production-concurrent; no TestMain in wave packages so shuffle has no init coupling)

## Verification

- [x] First `just ci` run failed on exactly the race the reviewer predicted — fix verified by 4 consecutive shuffled race runs and a clean second `just ci`

**PR:** (added once created)
