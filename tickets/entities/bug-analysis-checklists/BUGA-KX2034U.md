---
id: BUGA-KX2034U
type: bug-analysis-checklist
title: 'Analysis: rrule_next returns the after date itself for a rule without DTSTART'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`TestRruleNextOnTodaysOccurrence` and the no-DTSTART cases in `TestRruleNext` fail on develop)
- [x] Minimal reproduction steps documented (bug body: `rela.rrule_next("FREQ=DAILY", "<today>")` returns today)
- [x] Environment/conditions noted (any backend; a rule without DTSTART, `after` on an occurrence date; seen in the Atlas recurrence script)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (`luaRruleNext` delegates to `metamodel.NextRrule`)
- [x] Regression test planned (Lua tests: no DTSTART on its own occurrence, today-relative cases, DTSTART in both forms)
- [x] Related areas checked for similar issues (`predicatefns` already uses `metamodel.NextRrule`; feed and CalDAV code only validate rules with `rrule.StrToRRule`)
