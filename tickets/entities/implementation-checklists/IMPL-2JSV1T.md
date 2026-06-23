---
id: IMPL-2JSV1T
type: implementation-checklist
title: 'Implementation: ACL read-side: SSE /api/v1/_events per-type gating — type-scoped staleness signal, ReadQuery-gated'
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

- [x] Using fixture builders or factories for test data (reuses mustNewACL / gateCtxFor / seedEntity / the editor-of world from the VMD8/BA8BSX suite)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (driven via runSSELoop against an in-memory sink, reading the literal wire bytes)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Branch feat/acl-sse-tkt-pot9gq, 3 commits (0574e856 backend, bd355239 frontend,
87ccfb32 docs), stacked on PR 972.

- **Backend (internal/dataentry/sse_acl_test.go, 11 tests, all green):** AC1 per-type gating (read:[ticket] → ticket frame, no feature); AC2 role-relation Query verdict still delivers ticket; AC3 no-id-on-wire (asserts `TKT-`/`id` tokens absent from the byte stream); AC4 DenyAll withholds (no frame, no timing); AC5 debounce (burst of 20 → 1 frame) + multi-type (one frame per type); AC6 verdict cached (ReadQuery called once across 3 events) + relation-change invalidation (re-resolves → 2); AC7 fail-closed on zero/unresolvable verdict (withhold); non-entity passthrough (refresh/git delivered ungated to a denied principal); AC8 NopACL all types flow; TwoPrincipalsDifferentFrames (RR-GVHEIK — same store event, alice gets ticket, bob gets feature).
- **Rewritten existing tests:** TestSSE_DoesNotFlowAuditEvents (AC9) green; TestSSE_BroadcastEntityChange_CarriesTypeOnly asserts the new id-less shape; store-bridge tests updated for entity-change-by-type + the new relation-change marker (TestStoreEventBridgeRelationSignalsVerdictInvalidation); pg cross-process bridge test asserts id-less change.
- **Frontend (frontend, 1032 tests green, typecheck clean, lint 0 errors):** useEvents.ts consumes a single `entity:changed {type}` event (id-less), invalidates by type; useEvents.test.ts rewritten (11 tests); DocumentsPanel.vue + DocumentView.vue collapsed three per-op subscriptions to one `entity:changed`; production build succeeds (embedded SPA bundle regenerated).
- **Whole-tree:** `go build ./...` + `-tags postgres` + `-tags memorybackend` all compile; full dataentry suite green; `just ci` lint + arch-lint + tests green; coverage satisfied (sole failure is the gitignored e2e/node_modules artifact, absent in CI).
- **Design coverage:** every AC has a pinning test; the two carried-forward RRs (RR-K2WKEJ cached+invalidated verdict, RR-MTUW2N fail-closed) are pinned by AC6/AC7.

## Quality

- [x] Code follows project patterns (per-connection gate in handleSSE where the principal already lives; ReadQuery is the existing cheap verdict; non-entity events keep the existing pre-rendered path)
- [x] Checked for DRY opportunities — entityTypeVisible extracted as the single gate+cache point; the three create/update/delete pump cases collapse to one broadcastEntityChange; no premature abstraction
- [x] No security issues introduced — no id on the wire (per-type only); DenyAll withholds; fail-closed on gate error; audit-isolation preserved
- [x] No silent failures — a gate error fails closed (withhold) by design, which is the correct posture for a hint channel; relation events now surface (previously dropped) as the verdict-invalidation signal
- [x] No debug code left behind
