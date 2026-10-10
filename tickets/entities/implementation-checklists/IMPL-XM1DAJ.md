---
id: IMPL-XM1DAJ
type: implementation-checklist
title: 'Implementation: restish discovers rela''s spec via Link service-desc; CI e2e with real restish'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: the header is a constant; no error path)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The root-link test pins the literal header value on purpose: it is the wire
contract restish parses, so building it from `openAPISpecPath` would let a
change to both pass unnoticed.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- AC1: `TestOpenAPI_RootLinksToSpec` passes (`/` has the header, `/tickets` has none).
- AC2: `TestRestish_DiscoversSpecAndRoundTripsAttachment` passes locally against restish 2.3.0 (0.6 s). With `withSpecLink` removed it fails: `put-ticket-attachment` is not a command, because `api connect` found no spec.
- CI install step: the pinned archive's SHA-256 matches the release `checksums.txt`; the archive holds `restish` at its root, as the `tar` command expects.
- AC3: docs/restish.md regenerated from GUIDE-restish; the `spec_files` line is gone.
- CalDAV mount: `registerWellKnown` receives the wrapped handler, so `/` carries the header there too.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities: `openAPISpecPath` const now names the route in both the mux registration and the Link header
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
