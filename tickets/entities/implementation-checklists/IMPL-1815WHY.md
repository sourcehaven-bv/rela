---
id: IMPL-1815WHY
type: implementation-checklist
title: 'Implementation: IdP webhook: accept org-scoped events that carry no user_id'
started: "2026-11-07"
completed: "2026-11-07"
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: claims are small struct literals; the existing newWebhookTestApp helper builds the app)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: needs a live IdP sending signed webhooks; the handler tests drive the real receiver with a stub verifier, and real JWT verification is unchanged and covered in internal/jwtauth)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** AC1: TestWebhook_OrganizationEventReachesOrgHandler
and TestWebhook_DefaultOrgHandlerAcknowledges. AC2 and AC3:
TestWebhook_RequiredClaimsPerFamily (asserts the named claim). Edge cases:
TestWebhook_OrganizationEventWithUserIDStillRoutesToOrgHandler,
TestWebhook_OtherEventsReachAction, TestWebhook_OrganizationEventDedupAndRetry,
TestClassifyWebhookEvent. `go test -race ./internal/dataentry/` passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
