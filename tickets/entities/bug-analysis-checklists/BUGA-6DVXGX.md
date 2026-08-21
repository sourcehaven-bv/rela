---
id: BUGA-6DVXGX
type: bug-analysis-checklist
title: 'Analysis: Docs describe weekday schedules as ISO-week-change; code fires on target-weekday-passed'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (verified: `docs/scheduled-tasks.md:229` and `docs-project/entities/guides/GUIDE-scheduled-tasks.md:235` still say "ISO week changed"; `Schedule.IsDue` in `internal/scheduler/config.go` fires on target-weekday-passed)
- [x] Minimal reproduction steps documented (divergence table in the bug body: cases A and B disagree in both directions for a `friday` schedule)
- [x] ~~Environment/conditions noted~~ (N/A: static doc/code divergence, no runtime environment involved)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (docs-only: correct the bullet in the source-of-truth guide `docs-project/entities/guides/GUIDE-scheduled-tasks.md`, regenerate `docs/scheduled-tasks.md` via `scripts/generate-docs.sh`)
- [x] Regression test planned (`config_test.go` table cases pinning rows A and B: due on same-ISO-week weekday occurrence, not due after weekday passed even across ISO week boundary)
- [x] Related areas checked for similar issues (Day and Interval bullets in the same section match `IsDue`'s `dayKind`/`intervalKind` branches; the `Schedule` godoc and "Schedule Values" section already describe weekday semantics correctly)
