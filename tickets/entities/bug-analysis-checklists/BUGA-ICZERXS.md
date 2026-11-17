---
id: BUGA-ICZERXS
type: bug-analysis-checklist
title: 'Analysis: Long digit string in an integer property fails Postgres integer queries'
started: "2026-11-17"
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

Reproduced with `TestIntegerExprAgreesWithGo`: a string of 200,000 nines fails the old SQL with `value overflows numeric format`. Fix: accept at most 19 digits after leading zeros and strip the zeros and a `+` sign before the cast. The number branch reads jsonb's own numeric text, which always casts back, so it needs no bound.
