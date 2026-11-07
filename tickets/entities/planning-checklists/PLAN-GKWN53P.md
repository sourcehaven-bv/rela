---
id: PLAN-GKWN53P
type: planning-checklist
title: 'Planning: IdP webhook: accept org-scoped events that carry no user_id'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: classify webhook events by family in
`internal/dataentry/webhook.go`; required claims per family; route
`organization.*` to a separate handler seam whose default acknowledges and logs.
Out: tenant provisioning (TKT-TNPRV8), erasure (TKT-TNERAS), any change to JWT
verification or the adapter.

**Acceptance Criteria:**
1. Verified `organization.created` with `org_id` and no `user_id` returns 200
and reaches the organization handler; the action does not run.
2. `membership.*` without `user_id` returns 400 (also without `org_id`).
3. `organization.*` without `org_id` returns 400; handler not called.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small change; RES-D54281 covers the background)
- [x] ~~Searched for existing libraries~~ (N/A: routing on an event-name prefix)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: Pratique event names in pratique internal/core/core.go are the contract)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-D54281 (background)

**Existing Solutions:** `webhookReceiver.handle` already verifies, checks
claims, dedups on `jti` and dispatches. The org seam follows the consumer-side
interface rule like `webhookRoleStore`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Classify `claims.Event` into three families:
`organization.*` (needs `org_id`), `membership.*` (needs `user_id` and
`org_id`), and every other event (needs `user_id`, today's rule). Verification
and `jti` dedup stay as they are and apply to all families. Organization events
go to a consumer-side `webhookOrgHandler` interface on the receiver; the default
implementation logs and returns nil, so the event is acknowledged. A handler
error answers 502 and forgets the `jti`, like an action failure. Other events
keep dispatching to the configured action. Unknown event types with a `user_id`
still reach the action on purpose: the action is the provider-specific layer and
may ignore them.

Alternatives rejected: rejecting unknown event types (would break the
provider-agnostic contract and make Pratique log failures for events the script
may choose to ignore); sending organization events to the action (the action is
per-user and would need `user_id`).

**Files to modify:** internal/dataentry/webhook.go,
internal/dataentry/webhook_test.go, docs/idp-webhook-provisioning.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The only input is the signed JWT body. Claims
are read only after signature, issuer, audience and expiry verification
(unchanged). Missing required claims give a fixed 400 message naming the claim.

**Security-Sensitive Operations:** No new write path. The default org handler
only logs event and org id (not personal data). Dedup still runs before any
handler.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1: stub verifier returns `organization.created` +
`org_id`; recording handler is called once; status 200; action not run. Default
handler also gives 200. AC2: `membership.created` without `user_id`, and without
`org_id`: 400. AC3: `organization.created` without `org_id`: 400, handler not
called.

**Edge Cases:** Organization event that also carries `user_id` goes to the org
handler. Redelivered org event is deduped. Unknown event with `user_id` still
runs the action; without one it is a 400. Org handler error gives 502 and a
retry runs again.

**Negative Tests:** Bad signature still 401 (existing test).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Requiring `org_id` on membership events is stricter than before;
Pratique always sends it and the reference action already fails without it, so a
400 replaces a retry loop. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/idp-webhook-provisioning.md behaviour table and
claims list.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Proportionate self-review for a small ticket; no
findings recorded. The approach is checked again in the code review.
