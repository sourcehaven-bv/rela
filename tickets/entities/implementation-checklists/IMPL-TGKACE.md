---
id: IMPL-TGKACE
type: implementation-checklist
title: 'Implementation: Request-scoped Lua actions: full request in, arbitrary response out'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Both ends of the gap are closed, and both are opt-in:

- **Request in.** A `request:` block on an action projects the inbound call
into `rela.request` — `method`, `path`, `content_type`, plus `raw`/`body`
(`body: true`), `query` (`query: true`) and an ALLOWLISTED `headers` map.
`lua.Request` is a VALUE, not an `*http.Request`, so the lua package is never
handed a live request it could read a cookie or bearer token off.
- **Response out.** A script may return `{status, body, content_type}` instead
of the SPA's `{redirect, message, message_type}`. The two cannot be mixed:
that is a contract error, not a precedence rule, because the shapes are read by
different consumers and honoring one while dropping the other is a silent
half-delivery.

Integration coverage goes through the real handler rather than the units:
`actions_request_test.go` (16 tests) posts to the action endpoint and asserts
the wire result, and `actions_response_security_test.go` attacks the rich
response surface from the same direction.

Errors are surfaced, not swallowed, and the precedence is stated as a test
rather than left implicit: `TestAction_ScriptErrorBeatsScriptStatus` pins that
a script which FAILS gets rela's error envelope whatever status it was about to
return — a run that raised chose no status.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Tests build their app through the `newActionTestApp` helper and name only the
fields under test, so a change elsewhere in the config surface does not have to
be echoed across every case. `TestWithRequest_AllKeyIsNotShadowable` asserts
INSIDE Lua rather than through captured print output, so it does not depend on
how the runtime happens to wire stdout.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The motivating use case is implemented as a runnable artifact rather than
described: `examples/icinga-alert.lua` reads host/service/state from the posted
JSON, finds or creates the matching incident, appends the notification to its
markdown body, and answers with a status Icinga can act on.
`actions_example_test.go` executes that example through the handler, so the
documented example cannot rot into something that no longer runs.

*Nested-freeze regression, verified failing-first during this pass.* Reverting
the per-key and `_all` freezes in `internal/lua/request.go` makes
`TestWithRequest_ReadOnlyIsNotSkinDeep` fail on exactly two subcases
("the _all map" and "a per-key _all list"); restoring them returns the package
to green. This is the RR-J1L1FS defect, and the test is what stops it
returning — the earlier `TestWithRequest_ReadOnly` asserted only the top-level
table, which is precisely why the nested mutability survived review.

*Edge cases exercised at the wire:* a body over the cap is refused with 413 and
never truncated; a body that does not parse leaves `body` nil with `raw`
intact for a text or vendor payload, while the legacy non-request-scoped path
keeps its 400 on malformed JSON; `text/html` is refused; a CRLF in
`content_type` cannot split headers.

*Orthogonality:* `TestAction_CapabilitiesStillGatedWithRequestBlock` and
`TestAction_EntityIDStillResolvesWithRequestBlock` pin that a `request:` block
is not a back door — it grants no capability and is not a second route to an
entity the caller may not read (BUG-ZWTDH9's gate still applies).

`go test -race` green for `internal/lua`, `internal/script`,
`internal/dataentry` and `internal/dataentryconfig`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The request projection reuses the webhook path's reasoning and its ceiling
rather than inventing a parallel one: the same 1 MiB default, the same
`maxWebhookBodyCap` 8 MiB config-load ceiling, and the same read-at-limit+1
trick so an oversized body is DETECTED rather than truncated (truncated form
data parses fine and would have the script act on quietly-wrong values). The
header floor is likewise the one the declarative webhooks already enforce, so
an operator cannot get a weaker rule by choosing an action.

On security, this ticket's whole surface is attacker-influenced, and the
hardening is asserted rather than assumed — see the review checklist.
