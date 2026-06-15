---
id: PLAN-XCV0FK
type: planning-checklist
title: 'Planning: ACL read-side: SSE /api/v1/_events per-subscriber visibility — gate entity events at the subscriber'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: per-type ACL gating of the SSE `/api/v1/_events` feed. Entity events carry
`{type}` only (no id); each connection's `handleSSE` loop applies
`ReadGate.ReadQuery(type)` (DenyAll → withhold) before writing; the verdict is
cached per-connection and refreshed on membership-changing store events; bursts
are debounced/coalesced to one nudge per type per window; client invalidates
active queries by type (drops `data.id`).

OUT: per-id / per-query precision (future perf, not correctness); the
cacheId/HMAC scheme, master secret, mergebox, snapshot-ACL (all rejected —
captured in ticket + IDEA-CQMKMD); collaboration/presence; relation/attachment
SSE events; soft-delete/trash; MCP transport.

**Acceptance Criteria:** AC1–AC11 in TKT-POT9GQ. Test scenarios in Test Plan
below.

## Research

- [x] ~~/research~~ (N/A: a full design exploration ran in-conversation 2026-06-13 — per-id→mergebox→cacheId→use-case-reframe→per-type — captured in the ticket body's "Design arc"; plus web research on how Phoenix/Supabase/Meteor/PowerSync/AppSync handle ACL-filtered realtime feeds. The ticket body IS the research record.)
- [x] Searched for existing libraries — N/A (in-tree SSE broker; no external lib applies)
- [x] Checked codebase for similar patterns/reusable code
- [x] Looked for reference implementations — the realtime-ACL research surfaced the "authorize-on-subscribe / coarse staleness signal" pattern (Phoenix channels, Supabase Realtime) that this design follows
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (ticket body + IDEA-CQMKMD carry the full rationale +
rejected alternatives)

**Existing Solutions:**
- SSE broker: `internal/dataentry/watcher.go` — `eventBroker` (33), `handleSSE` (310, holds r.Context()/principal), `pumpStoreEvents` (183, sees all store events incl. the EventRelation* it currently drops), `broadcastEntityEvent` (74, pre-renders {type,id}).
- ACL read gate: `internal/dataentry/readgate.go` — `ReadQuery(ctx, type) → AllowAll|DenyAll|Query` (the cheap type-verdict; no per-entity walk).
- Routing: `router.go:118` attachACLRequest already wraps `/api/v1/_events`, so the principal+gate are in the connection ctx.
- Audit-isolation invariant precedent: `sse_audit_isolation_test.go` / watcher.go:162 godoc — establishes "SSE carries no principal-topology"; this ticket extends the same threat model to entity existence and carries even less (just a type).
- Fail-closed-on-gate-error pattern: `filterVisibleIncludes` (api_v1.go) and `StoreGraph.HasEdge` (treat-as-no-edge + Warn).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:** Full design in TKT-POT9GQ §"The design (final)".
Summary:
1. Broker change: entity events carry `{type}` (a small struct, not a pre-rendered `{type,id}` frame); non-entity events (`refresh`, `git:status`) keep the existing pre-rendered path.
2. `handleSSE` per-connection loop resolves `ReadQuery(type)` (cached per connection in a `map[type]verdict`), withholds on DenyAll, writes a `{type}` frame otherwise.
3. Cache invalidation: when `pumpStoreEvents` sees a member-of / role-relation write, signal connections to drop their cached verdicts (re-resolve lazily on next event). Membership-changing events are rare; ordinary entity writes don't invalidate.
4. Debounce/coalesce: a per-connection (or shared) short-window coalescer collapses a burst of same-type events into one nudge.
5. Client: `useEvents.ts` drops `data.id`, invalidates by `data.type` (≈ current behavior), debounces.

**Alternatives rejected** (detail in ticket): per-id (delete-can't-resolve),
mergebox (RR-LBXIB2 stale verdicts), cacheId/HMAC (heavy: master secret + DoS +
frontend map — over-engineered for a staleness signal), soft-delete (very-large
blast radius, doesn't solve cascade). Per-type chosen: cheap `ReadQuery` gate,
no id on the wire → no delete-leak/cascade/cacheId, over-fetch free at rela's
low write frequency.

**Files to modify:** TKT-POT9GQ §Files.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined (allowlist preferred)
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- The `type` on a store event is server-internal (from the store's own Event), never client-supplied — no injection surface. The SSE endpoint takes no body/query params that affect gating.
- Gating is allowlist by construction: a type is delivered only if `ReadQuery != DenyAll` (deny-by-default — an unknown/unresolvable verdict fails closed, AC7).

**Security-Sensitive Operations:**
- Per-type read gating (the point): `ReadQuery(ctx, type)`, cached, membership-refreshed (RR-K2WKEJ).
- Error handling: a gate error drops the nudge, keeps the connection (AC7, RR-MTUW2N) — fail-closed; never echoes the error (the payload is just `{type}` anyway; the error goes to slog).
- Residual leak (documented, accepted): per-type activity *timing* for types the principal can already read — strictly ≤ the pollable list-count signal. DenyAll principals get nothing.
- Audit-isolation (AC9): feed still carries no principal-topology.

## Test Plan

- [x] Test scenarios documented for each AC
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** `internal/dataentry/sse_acl_test.go` (new), driving store
events through gated SSE connections and reading the wire:
- AC1/AC4: read:[ticket] principal → gets {type:ticket}, not {type:feature}; DenyAll → nothing.
- AC2: alice/editor-of/PRJ-42 (Query verdict) → gets {type:ticket} nudges (not DenyAll), not {type:feature}.
- AC3: assert no entity id/slug anywhere in the byte stream for an entity event.
- AC5: burst of N same-type writes in a window → exactly one nudge.
- AC6: cached verdict + membership-change refresh — a principal removed from a conferring group stops receiving the type. Recording/spy on ReadQuery to assert it's not called per-entity on the hot path.
- AC7: injected ReadQuery error → no frame written, connection stays up.
- AC8: NopACL → all type nudges flow; client-shape (id-less) still invalidates.
- AC9: TestSSE_DoesNotFlowAuditEvents stays green.
- AC10 (frontend): useEvents.ts invalidates by type without id; debounce unit test.

**Edge Cases:**
- Two simultaneous connections, different principals → different nudge sets for the same store event (the C1/RR-GVHEIK pin).
- Empty/unknown type verdict → fail closed (withhold).
- Burst spanning multiple types → one nudge per type.
- Non-entity events (refresh, git:status) → always delivered, ungated.
- Connection drop mid-debounce-window → no panic, coalescer cleaned up.
- Membership-change event arriving → cached verdicts invalidated, next event re-resolves.

**Negative Tests:** AC4 (DenyAll withhold), AC7 (gate-error fail-closed), AC3
(no-id assertion).

**Integration approach:** handler-level tests driving real store events → SSE
wire bytes; frontend unit test for the invalidation/debounce. No new backend
conformance harness needed (this is dataentry-layer, not store-layer).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**
- Broker payload change (entity events structured vs pre-rendered) touches the shared broker — mitigated by keeping non-entity events on the existing path and pinning the two-principals-different-nudges test.
- Debounce window correctness (drop a real nudge / coalesce too aggressively) — mitigated by AC5 + a coalescer unit test; window is a hint-latency tradeoff, not correctness-critical (worst case = slightly delayed re-fetch).
- Cached-verdict staleness on membership change — mitigated by the pumpStoreEvents membership-event invalidation (AC6); window bounded.
- 3-deep stack rebase risk — `watcher.go` is untouched by 949/972, so conflicts unlikely; accepted.

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created on entering implementation

**Documentation Impact:**
- [x] GUIDE-acl-security (docs-project): SSE deferred→gated; per-type design + rejected alternatives + per-type-timing residual; threat-model → "all read channels gated"
- [x] docs/acl-security.md regenerated via `just docs`
- [ ] ~~docs/metamodel.md / cli-reference.md / data-entry.md / CLAUDE.md / README.md~~ (N/A: no metamodel/CLI/UI-surface change; the SSE payload shrinks, SPA behavior ≈ unchanged)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Two adversarial review passes (cranky-code-reviewer,
2026-06-13) + web research on realtime-ACL patterns. 11 RRs total. Resolved by
the final per-type design: RR-GVHEIK (broker plumbing, downgraded to significant
per user diagnosis), RR-K2WKEJ (cached cheap type-verdict + membership refresh),
RR-MTUW2N (fail-closed → AC7), RR-2LTVUL (DoS dissolved by per-type), RR-LBXIB2
+ RR-8CADXI (delivered-set abandoned). wont-fix/moot: RR-GB4UHX + RR-88NQA4
(cacheId machinery retired), RR-PZAQPB (no resume by design). The deep
exploration (per-id→mergebox→cacheId) is captured as rejected alternatives;
snapshot-ACL → IDEA-CQMKMD.
