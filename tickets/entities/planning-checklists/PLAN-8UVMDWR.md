---
id: PLAN-8UVMDWR
type: planning-checklist
title: 'Planning: Per-type records:read and records:append permissions'
started: "2026-11-23"
completed: "2026-11-23"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented

Scope: optional read/append permission per record type, global, narrows the
owner rule. Not in scope: per-owner permissions.

## Research

- [x] ~~Research doc~~ (N/A: follows the transition guard and tag permission pattern)
- [x] ~~Libraries~~ (N/A: internal)
- [x] Checked codebase for similar patterns (transitionGuard, HoldsPermission)
- [x] ~~Reference implementations~~ (N/A)
- [x] ~~Prior art in concepts~~ (N/A)

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered (per-entity check rejected: per-row cost on aggregates)
- [x] Dependencies identified

## Security Considerations

- [x] Input sources identified (schema.yaml names)
- [x] Input validation: names with whitespace rejected at load
- [x] Security-sensitive operations: all record service paths check it
- [x] Error handling does not leak: type-level check, names no entity

## Test Plan

- [x] Test scenarios documented
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

## Risk Assessment

- [x] Technical risks assessed
- [x] Security risks assessed
- [x] Effort estimated (m)

## Documentation Planning

- [x] User-facing docs identified (records guide)
- [x] Docs-checklist created

## Design Review

- [x] ~~Run /design-review~~ (N/A: small change on an existing pattern)
- [x] ~~Findings addressed~~ (N/A)
