---
id: PLAN-SQSTM7
type: planning-checklist
title: 'Planning: ACL read-side (PR 2/2): list endpoints + sidebar counts + pagination headers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope (verbatim from the ticket, verified against the code as TKT-VQGN landed
it):

- Gate `scopedSortedEntities` (api_v1.go:371) through the readGate: DenyAll short-circuits to empty BEFORE `freeTextIDsForType` / `applyV1Filters` / `applyV1Sorting`; Query path collects from `Store.GraphQuery` then runs search/filter/sort on the filtered slice; AllowAll keeps today's `listFromStoreByTypes` path byte-identical.
- All pagination leak surfaces (`meta.*`, `X-Total-Count`, `X-Page`, `X-Per-Page`, `Link` rel next/last) derive from post-filter `total` — automatic once scopedSortedEntities filters; pinned by AC3 tests.
- `sidebarCounts` single-mode collapse: drop the `typeCounts` precompute in `handleV1Sidebar` (api_v1.go:2430-2445); `countWithFilters` always goes through `gate.ReadQuery` + `GraphCount`/`GraphQuery`. `filterCache` stays.
- `acl.Policy.Validate` rejects read-deny + write-grant roles at load (write entry without covering read entry, wildcard-aware).
- Per-principal caching: existing `noCacheMiddleware` already sets `Cache-Control: no-cache, no-store, must-revalidate` on all `/api/` (covers the no-store requirement); add `Vary: <principal-header>` when `--principal-header` is configured (new `App.SetPrincipalHeader` + cmd/rela-server wiring).
- DenyAll list `_actions.create == false` — already structurally true via `computeCollectionActions` (write-path verdict) + the new policy invariant; pinned by AC4 test.
- Docs: GUIDE-acl-security read-path rewrite covering both PRs.

Out of scope: `_position` per-id semantics (RR-NDMN/RR-37IY/RR-ATSO follow-up —
note `_position` shares `scopedSortedEntities` so it inherits visible-subset
filtering implicitly, which keeps list/position coherent), `/_search` filtering,
SSE per-subscriber visibility, MCP transport, property redaction.

**Acceptance Criteria:** AC1–AC12 as documented on TKT-VMD8 (each maps to a test
in the Test Plan below).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: approach fully specified by the ticket, which is itself the output of multi-round design review with 16 RRs)
- [x] ~~Searched for existing libraries~~ (N/A: internal feature, no external library surface)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: pattern established in-repo by TKT-VQGN PR 1)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — TKT-VQGN's planning + RES round covered the two-PR
series.

**Existing Solutions:**

- `acl.Request.ReadQuery(ctx, type) ReadQueryResult` already exists (internal/acl/request.go:92) — errorless, returns AllowAll/DenyAll/Query. The dataentry `readGate` interface (readgate.go:32) just doesn't expose it yet; adding a third method follows the consumer-side-interface rule.
- `store.GraphQueryer.GraphCount(ctx, q) (matched, total, err)` (store/graphquery.go:63) is exactly the sidebar count shape; graphquerynaive backs all three backends.
- Test harness reuse: `mustNewACL`, `gateCtxFor`, `principalCtx` from acl_get_test.go; `stripInstance` comparator pattern from PR 1.
- Error mapping reuse: `writeGateError` (api_v1.go:794) for ACL-query failures on the list path.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `readgate.go`: add `ReadQuery(ctx, entityType) acl.ReadQueryResult` to the `readGate` interface. `aclReadGate` delegates to `req.ReadQuery`; `nopReadGate` returns `{AllowAll: true}`.
2. `scopedSortedEntities`: resolve `rqr := readGateFromContext(ctx).ReadQuery(ctx, typeName)` first. DenyAll → return empty immediately (search backend untouched). Query → collect `Store.GraphQuery(ctx, *rqr.Query)`; iterator error wrapped in a sentinel (`errACLListQuery`) so call sites route it through `writeGateError` instead of the `search_failed` 500. AllowAll → existing path unchanged. Search/filter/sort run after, on the (possibly filtered) slice — pins search-after-ACL ordering.
3. `handleV1ListEntities` + `handleV1EntityPosition` call sites: distinguish sentinel-wrapped ACL errors (→ `writeGateError`) from search errors (→ existing `search_failed`).
4. Sidebar: delete `typeCounts` precompute; `countWithFilters` becomes gate-aware: DenyAll → 0; no config filters → `GraphCount` (AllowAll uses bare `GraphQuery{EntityType: t}`, Query uses `*rqr.Query`), matched count; with config filters → collect entities (GraphQuery), `applyFilters`, `len`. Errors degrade to 0 (parity with today's CountEntities error path) with `slog.Warn`.
5. `acl/policy.go` `Validate()`: for each role, every `Write` entry must be covered by `Read` (exact or `*`; `write: ["*"]` requires `read: ["*"]`). Structured error names the role and type.
6. `App.principalHeader` field + `SetPrincipalHeader`; `noCacheMiddleware` adds `Vary` when set; cmd/rela-server wires the flag value.

**Files to modify:** as listed on the ticket; plus
`internal/dataentry/readgate.go` (interface), `internal/dataentry/watcher.go`
(noCacheMiddleware Vary), `internal/dataentry/app.go` (SetPrincipalHeader),
`cmd/rela-server/main.go` (wiring), `internal/dataentry/acl_get_test.go`
(fakeGate gains ReadQuery).

**Alternatives rejected:**
- Exposing `Globals`/role internals to dataentry instead of `ReadQuery` — leaks policy vocabulary across the boundary; ReadQueryResult is the designed seam.
- Filtering after load via `PermitsReadMany` on the full id list — O(N) MatchingIDs work duplicating what GraphQuery does in one pass, and leaves the search backend running pre-ACL.
- `Cache-Control: private` addition — redundant: `no-store` (already set by noCacheMiddleware) is strictly stronger; only `Vary` adds defense.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Query params (page/per_page/q/filter/sort): unchanged, already clamped; they now operate on the post-ACL slice only.
- `acl.yaml`: new load-time invariant (write⊆read) is an allowlist-shaped check; structured error names role+type, no data leak.
- Principal header: already sanitized (sanitizeUser); only its NAME (operator config, not request data) goes into `Vary`.

**Security-Sensitive Operations:**
- Cardinality leak surfaces: every count/header derives from one post-filter `total`; AC3 enumerates all eight.
- Timing side-channel (RR-X56H): DenyAll returns before the search backend is touched — pinned by mock-searcher call-count test.
- Cache cross-principal leak (RR-VDTW): no-store already global on /api/; Vary added when principal header configured.
- Fail-loud preserved: ACL query errors surface via writeGateError (500/504), never silently as empty list — an empty-on-error list would be deny-shaped but masks outages.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** (files per ticket: acl_list_test.go, acl_sidebar_test.go,
acl_list_regression_test.go, acl/policy_test.go)
- AC1: viewer role read:[ticket] → GET /api/v1/tickets lists all tickets; GET /api/v1/features → data:[].
- AC2: role-relation `editor-of` confers read via inherit_roles_through:[belongs-to]; only TKT linked under the granted project returned.
- AC3: 5 visible + 5 hidden, per_page=3&page=2 → data.length=2, meta.total=5, has_more=false, X-Total-Count=5, X-Page=2, X-Per-Page=3, Link last→page=2, no rel="next"; assert "10" appears nowhere.
- AC4: no-read principal → data:[], meta.total=0, headers 0, `_actions.create == false`.
- AC5: recording searcher injected; DenyAll list request → searcher call count 0.
- AC6/AC7: sidebar counts = visible subset; with config filter = visible∩filter; same path with and without acl.yaml (single-mode).
- AC8: policy fixture write:[ticket] without read → LoadPolicyBytes error naming role+type; wildcard cases (write:* + read:* ok, write:* + read:[ticket] rejected).
- AC9: wrapper store records GraphQuery call; recording searcher records search call; assert order GraphQuery < search and search input scoped to filtered slice.
- AC10: router-level test with principal header configured: response has Cache-Control no-store + Vary: <header>.
- AC11: NopACL (no acl.yaml): list + sidebar responses carry full expected JSON (golden-style assert), proving no shape drift.
- AC12: docs review item in the docs checklist.

**Edge Cases:**
- Empty type (0 entities) under each verdict — counts 0, no Link next, last→page=1.
- page beyond last visible page → empty data, correct headers.
- GraphQuery iterator error mid-stream → 500 acl_query_failed (not partial list).
- ctx.Canceled during ACL query → no response written (writeGateError parity).
- Role with write:[] and read:[] (empty both) → valid policy, DenyAll.
- Sidebar list config referencing unknown list id → existing behavior unchanged (no count).
- per_page=100 cap, page=0 floor — unchanged, now post-ACL.

**Negative Tests:** AC5 (searcher must NOT be called), AC8 (policy load must
fail), AC3 (rel="next" must be absent; "10" must not appear in any header/body
field).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Base PR 939 not yet merged — branch stacks on feat/acl-readside-tkt-vqgn; if review forces changes to PR 1, rebase this branch (cherry-pick pattern already proven twice in this stack).
- `_position` behavior changes implicitly (visible-subset ordinals). Mitigation: this is the coherent choice (same pipeline as list); documented in PR + ticket; per-id gate semantics remain the follow-up's scope.
- Policy-load invariant may reject existing operator policies that relied on write-without-read. Mitigation: structured error tells the operator exactly which role/type to fix; documented in GUIDE-acl-security; this rejection is the designed outcome of RR-W2J6.
- Sidebar perf on large visible sets with config filters (RR-REQW): documented caveat only, no code change; follow-up ticket named.

Effort: m (matches ticket).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- GUIDE-acl-security (docs-project) — full read-path section rewrite (AC12).
- docs/acl-security.md — regenerated from the guide.
- docs/security.md — note the write⊆read policy invariant if it documents role grants.
- N/A: metamodel.md, cli-reference.md (no command changes).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** This ticket's scope IS the output of the
TKT-VQGN/TKT-VMD8 multi-round design review: RR-X56H, RR-W2J6, RR-VDTW, RR-REQW
(deferred from PR 1 to here), RR-2O27, RR-3IO2, RR-BZ4M, RR-FF7Q, RR-KNGC,
RR-WX77, RR-Q5LH all have their resolutions baked into the Scope/AC text above.
RR-NDMN, RR-37IY, RR-ATSO carry to the _position follow-up. No new design review
round needed — re-running it on the same reviewed text would be circular.
