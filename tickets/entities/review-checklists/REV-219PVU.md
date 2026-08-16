---
id: REV-219PVU
type: review-checklist
title: 'Review: Vulnerability Check red on develop — CI go-version pin resolves to an unpatched 1.26.5'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — full suite green
- [x] Lint clean — `golangci-lint` 0 issues
- [x] Coverage maintained — no Go source changed; the fix is a toolchain pin

`./scripts/govulncheck-filtered.sh` exits 0 with only GO-2026-4923 ignored (no upstream fix), down from 8 actionable findings.

## Code Review

- [x] Run `/code-review` command — reviewed on PR #1338 and approved without findings
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — none raised
- [x] Self-reviewed the diff for unrelated changes — 20 `go-version` entries, one `toolchain` directive, two dependency bumps

## Verification

- [x] Acceptance criteria met — `Vulnerability Check` green on `develop`
- [x] Manual testing performed — proved the findings pre-existed using a clean worktree before changing anything, and confirmed all 8 were fixable rather than ignorable
- [x] No regressions introduced — cross-compile matrix green on all six targets
- [x] Documentation updated — the measure is recorded as AM-exact-go-patch-pin

## Retrospective note

Recorded after the fact. This bug merged without a linked review checklist
because the ticket gate did not run on its PR (see BUG-CI7XKP); the gate is
repo-wide, so the gap surfaced on the next PR that did run it. Written now
rather than back-dating a claim that the checklist was followed at the time.
