---
id: BUGA-66SCBNL
type: bug-analysis-checklist
title: 'Analysis: An empty enum reads None where other empty fields read Empty'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (InlinePropertyValue.test.ts 'words an empty value the same for an enum, an enum list and a string' fails on develop with None, None, Empty)
- [x] Minimal reproduction steps documented (open an entity page you may edit with an empty string field and an empty enum: the string reads Empty, the enum reads None)
- [x] Environment/conditions noted (any backend; the inline edit of an entity page section, for a user with update rights)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (the pick and multi pickers and the empty enum option use the library emptyValue text that RlInlineEdit already shows; common.none is no longer used and is removed)
- [x] Regression test planned (the two new InlinePropertyValue.test.ts cases)
- [x] Related areas checked for similar issues (read-only display renders an empty value blank, as dense cells do; the relation picker's none and list grouping's (none) are other states, not an empty value)
