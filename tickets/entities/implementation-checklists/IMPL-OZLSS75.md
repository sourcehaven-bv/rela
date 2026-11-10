---
id: IMPL-OZLSS75
type: implementation-checklist
title: 'Implementation: tenant: choose the tenant for anonymous requests (or decide there is none)'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: decision and docs only, no code change)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: decision and docs only, no code change)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: decision and docs only, no code change)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: decision and docs only, no code change)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: decision and docs only, no code change)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: decision and docs only, no code change)
- [x] ~~Only specifying values that matter for the test~~ (N/A: decision and docs only, no code change)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: decision and docs only, no code change)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: decision and docs only, no code change)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: decision and docs only, no code change)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: decision and docs only, no code change)

**Verification Evidence:** DEC-9EZ3E8H exists, is accepted and decides
authorization and data-entry-server. `just docs` regenerated
docs/postgres-backend.md with the new paragraph.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: decision and docs only, no code change)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: decision and docs only, no code change)
- [x] ~~No debug code left behind~~ (N/A: decision and docs only, no code change)
