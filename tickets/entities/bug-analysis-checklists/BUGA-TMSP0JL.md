---
id: BUGA-TMSP0JL
type: bug-analysis-checklist
title: 'Analysis: Seed script on the fs backend hits the 30 s rela script timeout'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

## Notes

- Reproduced on a copy of the GRC demo config in /tmp (demo project and its
  database not touched), fs backend: develop stops at about 220 entities with
  `scripts/seed.lua:35: context deadline exceeded`.
- Wall-clock profile on develop: about 45% of the run inside bleve
  `EntityPut` (batch persist plus a second `SetInternal` for LastModified),
  about 34% in fsynced markdown writes, and the id scan
  (`collectAllIDs` reading every entity file) on every create.
- CPU (user+sys) for the full seed: develop 9-10 s, fix 5.8-6.3 s. Wall time
  is mostly fsync waits; the machine load was 15-45 from other sessions, so
  wall numbers vary.
- Related: TKT-69TDK8 (backlog) covers the same scan for pgstore and short
  ids; it stays open. BUG-UR0520H covers the remaining fsync cost and the
  fixed script timeout.
