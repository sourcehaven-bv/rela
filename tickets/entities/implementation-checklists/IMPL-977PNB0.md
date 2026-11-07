---
id: IMPL-977PNB0
type: implementation-checklist
title: 'Implementation: Store IdP-asserted roles per principal for background jobs (R14)'
started: "2026-11-06"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: a live run needs a signing IdP; TestWebhook_StoresAssertedRoles drives POST /webhooks/idp through the router and TestScheduledForEachPrincipal_StoredRoles opens an ACL request on the minted principal)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Webhook: roles stored for the verified subject only; empty list clears;
  error response, failing script, hold failure and store failure give 502;
  other actions returning `asserted_roles` give 500.
- Store: damaged or unknown-version document is an error for reads and
  writes; concurrent sets lose nothing; per-subject hold serializes.
- for_each: the child principal carries stored roles and holds the mapped
  rela role; shared, missing or moved principal values grant nothing.
- Lua: `rela.principals_with_role` returns sorted readable user ids, drops
  hidden, missing and other-type ids, and raises for an undeclared role or
  without the capability. TestScriptReads_PrincipalsWithRoleGated runs it
  through the real declarative read gate.
- Audit: each store or clear writes one `idp-roles-set` record naming the
  bound entity and the role count, never the IdP subject
  (TestIDPRoleWriter_BindsTheResolvedEntity).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
