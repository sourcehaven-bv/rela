---
id: IMPL-EW15M8D
type: implementation-checklist
title: 'Implementation: Point to RELA_DATAENTRY_USER on acl_unstamped_principal'
started: "2026-10-30"
completed: "2026-10-30"
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: table of flag values; the middleware test reuses newTestAppV1)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Adapted after BUG-C8DHGF (#2032) landed on develop during review. A request without a user now gets 401 `unauthenticated` instead of 500 `acl_unstamped_principal`, and 500 is left for wiring faults. The identity hint therefore moved to the 401 detail: "no user identity: local $RELA_DATAENTRY_USER, or rela-server --jwt-* or --principal-header; see docs/server-security.md". The 500 detail is back to its original text, and the startup warning says "every /api/ request will be refused with 401 unauthenticated". TestACLMiddleware_AnonymousIsUnauthenticated asserts the hint, and the golden decl-header/anonymous.golden was regenerated. A manual rela-server run on acl.yaml without identity logged the warning once and answered 401 with that detail.
- Earlier evidence, before the adaptation:

- Built rela-server and ran it on a project with acl.yaml and no identity source. Startup logged `level=WARN msg="acl.yaml is configured but no identity source is set: every /api/ request will fail with acl_unstamped_principal" fix="configure --jwt-* (or --principal-header behind an authenticating proxy), or set $RELA_DATAENTRY_USER for local use" docs=docs/server-security.md`. `curl /api/v1/person` answered 500 `acl_unstamped_principal` with detail "no identity: local $RELA_DATAENTRY_USER, or rela-server --jwt-*/--principal-header; see docs/server-security.md" (final wording after code review; the warning is logged once, at startup).
- Same project with `RELA_DATAENTRY_USER=alice`: no warning, and the request was not refused as unstamped.
- TestACLMiddleware_FailLoudOnApi asserts the detail names RELA_DATAENTRY_USER, --principal-header and --jwt-*. TestWarnUnidentifiedACL covers the warning rows (no source; blank env user) and the no-warning rows (no acl, principal header, env user, JWT with and without acl). TestValidateConfigEditing gained a blank env user case for the shared hasIdentitySource.
- TestNoInternalErrorDetail requires a 500 detail to be one string literal, so the detail is a single literal kept within the 120-column limit. The golden corpus (decl-header/BUG-C8DHGF-anonymous.golden) was regenerated with `-update`; the only change is the detail line.
- The non-loopback $RELA_DATAENTRY_USER warning (added after the security review) sits in main()'s startup block, which has no test harness; it was not run manually because that needs a non-loopback bind.
- go test ./internal/dataentry/... ./cmd/rela-server/...: ok. golangci-lint on those packages: 0 issues. `just arch-lint`, `just comment-lint`: clean. `just docs` regenerated docs/server-security.md and docs/acl-security.md.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
