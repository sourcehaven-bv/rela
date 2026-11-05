---
id: BUGA-ZYKMWJV
type: bug-analysis-checklist
title: 'Analysis: Live view misses a change reverted before the next round'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestLive_LeaseClaimedAndReleasedBetweenRoundsFrames and TestLive_WriteRevertedBetweenRoundsFrames fail on develop, every run)
- [x] Minimal reproduction steps documented (subscribe; change; client GET; revert; tick: no frame)
- [x] Environment/conditions noted (manual round clock in live_served_test.go; in E2E the 500 ms window makes it rare)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (record served response hashes; design reviewed)
- [x] Regression test planned (live_served_test.go)
- [x] Related areas checked for similar issues (presence, channel, inbox and busy subscriptions share commitRound and get the same fix)
