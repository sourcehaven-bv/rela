---
id: BUGA-RC0T3G
type: bug-analysis-checklist
title: 'Analysis: Conversion dialog disappears on Review'
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

A copy of the atlas config on the filesystem store, one seeded `topic` and a
principal with the `ciso` role. `POST
/api/v1/_conversions/topic-naar-opdracht/preflight` for that topic returns
`required`, `relations_to_drop` and `invalid_relations` as `null`.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

See why1 to why3 on the bug.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Send empty arrays from the handler and normalize on the client. Task-item
conversions use a different response without list fields that can be null.
