---
id: IMPL-6BQQ3C
type: implementation-checklist
title: 'Implementation: ACL-hidden properties leak through _views section field values, and render as editable-but-403 in inline edit'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `TestACLViews_RedactsHiddenPropertyValue` / `...PrimaryTitle`.
- [x] Integration tests written (test full flow, not just units) — tests drive `handleV1Views` end-to-end through the real gate + section builders.
- [x] Happy path implemented — `executeView` redacts entry + collections via `viewReader`.
- [x] Edge cases from planning handled — traversal + `where:` filters run on raw entities (redaction deferred to the boundary); entry re-gate avoided (already gated at the handler).
- [x] Error handling in place — `NewPolicyReader` wiring returns error in `NewApp`; `Filter` fails closed on gate error (logged).

## Test Quality

- [x] Using fixture builders or factories for test data — `newTestAppV1` / `seedEntity` / `mustNewACL`.
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: assertion is a leak-check for a sentinel string, which must be a literal).
- [x] Only specifying values that matter for the test — a hidden property + a distinctive value; nothing else.
- [x] ~~Interpolated values constructed from objects~~ (N/A: sentinel leak check).
- [x] ~~Property comparisons use original object~~ (N/A: sentinel leak check).

## Manual Verification

- [x] Feature manually tested end-to-end — neutralized the redaction, confirmed both tests fail (leak reproduced), restored, confirmed they pass.
- [x] Each acceptance criterion verified — hidden value absent from `_views` body; hidden primary falls back to id.
- [x] ~~Edge cases manually verified~~ (N/A: covered by the neutralize/restore round-trip and the existing suite).

**Verification Evidence:** Without the fix,
`TestACLViews_RedactsHiddenPropertyValue` and `...PrimaryTitle` FAIL with
`SECRET-STATUS` / `SECRET-TITLE` visible in the response body. With the fix,
both PASS. Full `internal/dataentry` + `internal/visibility` suites pass under
`-race`; `golangci-lint` 0 issues; `just arch-lint` clean.

## Quality

- [x] Code follows project patterns — mirrors the export handler's `visReader` wiring (DEC-ZBI39P).
- [x] Checked for DRY opportunities — extracted the thrice-duplicated `affRedactor` construction into `App.redactor()`.
- [x] No security issues introduced — the change removes a disclosure; redaction fails closed.
- [x] No silent failures — wiring error is returned; `Filter` gate errors are logged.
- [x] No debug code left behind — the temporary neutralization was reverted.
