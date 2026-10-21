---
id: IMPL-KFI5W8
type: implementation-checklist
title: 'Implementation: End SSE streams when the verified token expires'
started: "2026-10-20"
completed: "2026-10-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestVerifyAssertion_ProjectsExpiry`, `TestVerifyAssertion_ExpiryIncludesLeeway` (jwtauth), `TestAssertedIdentity_CopiesEveryField` (cmd/rela-server), `TestSSEConn_WriteDeadlineCappedAtExpiry` (dataentry)
- [x] Integration tests written (test full flow, not just units): `internal/dataentry/sse_expiry_test.go` drives the real router with the JWT gate and a stub verifier in a `synctest` bubble: `TestSSEExpiry_StreamEndsAtExpiry` (both stream routes), `TestSSEExpiry_NoExpiryStaysOpen` (no gate, and the gate with a zero expiry), `TestSSEExpiry_PastExpiryWritesNothing`, `TestSSEExpiry_RefreshedTokenKeepsStreamOwner`
- [x] Happy path implemented: the gate records the assertion expiry on the request context; the stream ends at it and the live connection is unregistered
- [x] Edge cases from planning handled: expiry already past (no frames, no registration), zero expiry (no timer), a blocked write (deadline capped at the expiry), the legacy `/api/events` stream, a refreshed token on a live PUT
- [x] Error handling in place (errors surfaced, not swallowed): ending at expiry is a normal stream end; a write that hits the capped deadline returns its error and ends the loop as before

## Test Quality

- [x] Using fixture builders or factories for test data: the existing stream helpers and a per-call stub verifier
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the synctest tests drive the real router, gate and stream loop; a manual check would need an OIDC proxy minting short-lived tokens)
- [x] Each acceptance criterion verified with test scenario from planning: AC1 `TestSSEExpiry_StreamEndsAtExpiry`, AC2 `TestSSEExpiry_NoExpiryStaysOpen`, AC3 `TestSSEExpiry_PastExpiryWritesNothing`, AC4 `TestVerifyAssertion_ProjectsExpiry` and `TestAssertedIdentity_CopiesEveryField`, AC5 `TestSSEExpiry_RefreshedTokenKeepsStreamOwner`
- [x] ~~Edge cases manually verified~~ (N/A: covered by the tests above)

**Verification Evidence:** build and vet on the default, `postgres` and `sqlite`
tags; `go test -race` on internal/jwtauth, internal/dataentry and
cmd/rela-server; the new tests with `-race -count=20`; golangci-lint on the
changed packages (0 issues); `just arch-lint`, `just comment-lint` and `just
docs` pass. With the expiry checks disabled, AC1 and AC3 fail.

## Quality

- [x] Code follows project patterns (check similar code): the expiry rides an unexported context key like `replayPrincipalKey{}`; the stream loop gains one timer case next to `flush` and `ping`
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one `assertionLeeway` constant feeds both
the parser and `ExpiresAt`; the adapter copy moved to `assertedIdentity` so one
test covers it
- [x] No security issues introduced: rela-security-reviewer found one nit (the deprecated resolver), addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
