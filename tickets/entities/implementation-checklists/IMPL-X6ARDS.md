---
id: IMPL-X6ARDS
type: implementation-checklist
title: 'Implementation: Webhook principals get principal_type webhook; acl map --webhook with --expect CI gate'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (principal/webhook_test.go, acl/clampedgrants_test.go, aclmap/webhook_test.go, aclaudit/webhook_test.go)
- [x] Integration tests written (test full flow, not just units) (dataentry/webhook_ceiling_test.go posts real deliveries through an ACL-wired app; cli/acl_map_webhook_test.go runs the command)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no baseline unchanged, write outside baseline, unreadable find type, stale ctx Request, unknown hook id, flag conflicts, invalid expect file)
- [x] Error handling in place (errors surfaced, not swallowed) (refused automation writes are logged by ingest)

## Test Quality

- [x] Using fixture builders or factories for test data (hookACLApp, hookServices, hookWorld)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built rela and ran it on a demo project with the Atlas-like policy (role
icinga-webhook, baseline applies_to [webhook] read/create/update incident,
deny_delete and deny_permissions "*"):

- `acl map --webhook icinga-alert --expect expect.yaml`: capped access,
"access matches", exit 0.
- Same with a narrower file: 3 differences listed, exit 1.
- `acl map --webhook nope`: "unknown webhook", exit 1.
- Without the baseline: `acl audit --fail-on=medium` reports
A15-webhook-uncapped and exits 1; the map header says no baseline matches.

Mutation check: removing `acl.WithRequest(ctx, nil)` from the route fails
TestWebhookRoute_ClearsAttachedRequest. Finding: declarative automation actions
are system writes and stay uncapped; Lua actions are capped. Documented in
docs/webhooks.md and the plan.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (MapPrincipalAs delegates to
MapPrincipalFull; acl re-exports PrincipalTypeWebhook instead of a second
literal)
- [x] No security issues introduced (the type only narrows; system: names cannot be asserted)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
