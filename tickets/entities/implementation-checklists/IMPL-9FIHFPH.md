---
id: IMPL-9FIHFPH
type: implementation-checklist
title: 'Implementation: Tenant DSN derivation drops sslmode and other base_dsn settings, downgrading TLS to prefer'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

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

- [x] ~~Feature manually tested end-to-end~~ (skipped: verified by automated tests only, no manual run against a live server)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (skipped: covered by the targeted tests listed below, not by hand)

**Verification Evidence:** Commit f555d7ea7. `TestDSNHelpers_KeepBaseSettings`
compares parsed TLS and pool config of base and derived DSN;
`TestPinSearchPath_RefusesOtherSpellings`,
`TestPinSearchPath_OverridesBasePath`; tenant and docscapture callers use the
helpers.

Targeted tests pass on the default, postgres (-race) and sqlite build tags. The
full `go test -race` suite passed except sqlitestore, which timed out under load
and passes when run alone.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
