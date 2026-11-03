---
id: PLAN-XZANKT
type: planning-checklist
title: 'Planning: Realtime 2/1d: lease HTTP routes, _lease on responses, live wake'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding
- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**In scope (1d / 1c-HTTP):** POST /api/v1/{plural}/{id}/_lease, POST
/api/v1/_leases/renew, POST /api/v1/_leases/release, GET /api/v1/_leases;
`_lease` on entity GET and list rows after row gating; lease changes wake the
live hub; ETag/304 rule; dataentry.SetLeases wiring through dataentrywire;
docs/data-entry/api-reference.md and GUIDE-acl-security.md. **Out:** MCP
(TKT-27XVYS), SPA badge (TKT-MHXH87), CLI (TKT-B610IP).

**Acceptance criteria:**
1. Claim on hidden, nonexistent, wrong-plural and not-wired: 404 with a body identical to the entity GET 404 (writeWriteTargetError); bare address on a faced type: 422 face_required.
2. Read without update: 403 without holder fields; `_actions.update == false` implies claim 403 (affordance contract case).
3. Held: 409 lease_held with holder {user, tool, label}, since, expires_at, status.
4. Renew after expiry, revocation, delete, delete+recreate: 409 lease_lost; release: 204 always; limit: 409 lease_limit; store full: 503 with Retry-After.
5. `_lease{user,tool,label,since,status}` on entity GET and list rows only for readable rows, absent when not held, never set by entitySerializer; a hidden fixture through views, history and includes has no `_lease`.
6. List cost: one lease read per page, same count at 10 and 50 rows (counting decorator).
7. A plain renew leaves the live hash unchanged; claim, status change, release and expiry each produce exactly one live:changed.
8. TestLive_LeaseOnHiddenEntitySendsNoFrame.
9. 304 is skipped while a lease is present; If-Match keeps working on the unchanged ETag.
10. lease_id never appears in GET, list, SSE frames or captured logs (including the renew/release error paths); request logging never logs bodies.
11. A cross-origin POST with a cookie is refused through NewRouter; /api/v1/_leases/ is not CSRF-exempt.
12. router_walk and openapi route tests cover the new routes.

## Research
- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: covered by RES-1UCUBA)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-repo HTTP surface)
- [x] Checked codebase for similar patterns or reusable code (pilesHandler, serveListRowExtras/comment counts, writeWriteTargetError, live hub feed loop)
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art (authorization, uniform 404, collection reads)

## Approach
- [x] Technical approach chosen and documented (.ignored/phase2-plan.md "1c design" 1c-HTTP and the folded security review)
- [x] Approach builds on existing patterns (not reinventing) (pilesHandler, package-level setter because App is at its plimsoll cap)
- [x] Alternatives considered (document why rejected) (new `lease` _actions verb: second name for the update decision; 410 for lost: no resource in the URL; 429 for limits: retry does not help; lease in ETag: every claim would 412 edits; targeted live replays by id: subscriptions are URLs)
- [x] Dependencies identified (packages, APIs, types) (lease.Service, dataentrywire, v1 responses, live hub)

## Security Considerations
- [x] Input sources identified (user input, config, external APIs) (route plural/id, JSON bodies: ttl_seconds, label, status, lease_id)
- [x] Input validation approach defined (allowlist preferred over blocklist) (service validators; strict JSON decoding)
- [x] Security-sensitive operations identified (file access, auth, crypto) (row gate, update authorization, lease_id secrecy, CSRF)
- [x] Error handling doesn't leak sensitive information (shared 404 writer; held only after read+update; no lease_id in logs)

## Test Plan
- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented (wrong plural, faced bare address, renew with absent vs empty status, not-wired service)
- [x] Negative test cases defined (invalid input, error conditions) (hidden/nonexistent/forbidden/held/lost/limit/full/malformed JSON/cross-origin)
- [x] Integration test approach defined (not just unit tests) (NewRouter with a real lease service over a temp project with acl.yaml; live SSE tests)

## Risk Assessment
- [x] Technical risks assessed with mitigations (live hub wake cost: rounds stay on the fixed clock and capped; shared v1.Entity: single attachment points with a test)
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) (m)

## Documentation Planning
- [x] User-facing docs identified (skip if internal refactor) (docs/data-entry/api-reference.md; GUIDE-acl-security.md regenerated into docs/)
- [x] Docs-checklist will be created when entering implementation
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: SPA in TKT-MHXH87)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: user-facing API)

## Design Review
- [x] Run `/design-review` before starting implementation (security design review of 1c covered both halves; HTTP findings recorded on this ticket)
