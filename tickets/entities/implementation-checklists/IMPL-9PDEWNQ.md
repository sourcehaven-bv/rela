---
id: IMPL-9PDEWNQ
type: implementation-checklist
title: 'Implementation: Metric capture and trend charts over records'
started: "2026-11-16"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Built rela-server and ran it on a demo project with a
`metric` type and a `measurement` record type. Appended 22 months of rows
through `POST /api/v1/_records/measurement`. The dashboard showed a three-owner
line card (legend with titles, end labels, crosshair tooltip on hover, table
view), a one-owner card, and a card whose only owner does not exist, which
showed "no data" after the aggregate answered 404. Checked in light and dark
mode. Hidden-owner behaviour is covered by the service test (per_owner case) and
DashboardRecordsChart.test.ts; the Lua owner gate by appbuild
TestRecords_LuaAppendUsesOwnerGate with a real acl.yaml.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
