---
id: TKT-VMD8
type: ticket
title: 'ACL read-side (PR 2/2): list endpoints + sidebar counts + pagination headers'
kind: enhancement
priority: medium
effort: m
status: done
---

Second of a two-PR read-side ACL series. Builds on the per-entity gate landed in
TKT-VQGN (PR 1) and extends ACL enforcement to list-style endpoints and the
aggregates that mirror them. `_position` is list-derived but deferred to a
follow-up ticket so PR 2 stays focused on the list response shape and sidebar
aggregates.

## Why this split

Per-entity-response gates (TKT-VQGN) and aggregate gates (this ticket) have
different threat surfaces: per-entity needs deny-shape parity with not-found;
aggregates need pagination/cardinality/menu-shape parity with empty-or-filtered.
Splitting keeps each PR's deny model coherent and tightly tested.

This PR's narrow goal: **anything that enumerates entities of a type returns
only the visible subset, with no leak surface that reveals hidden cardinality.**

## Prerequisite

Stacks on TKT-VQGN's PR. TKT-VQGN ships the `readGate` consumer-side interface
and the dataentry-side wiring patterns this PR consumes:

- `readGate` interface (`internal/dataentry/readgate.go`) with `Visible` and `Query` methods, retrieved via `readGateFromContext(ctx)`.
- `*acl.Declarative` constructed with a `GraphQueryer` (so `Query` results can be executed).
- Middleware fail-loud invariant on `/api/` paths.
- Structural-equal JSON-canonical comparator helper for regression tests.

PR 2 does not strictly NEED `Visible` or `WhereIDs` for its own scope, but
stacking on PR 1 minimizes rebase pain on the shared files (`api_v1.go`,
`router.go`, `handlers_api.go`) that both PRs touch.

## Scope (this ticket / single PR)

### Read path

- **`internal/dataentry/api_v1.go` — gate `scopedSortedEntities`** (line ~371). After resolving `typeName`, get `gate := readGateFromContext(ctx)` and call `gate.Query(ctx, typeName)`:
  - `AllowAll` → today's path (no change).
  - `DenyAll` → **short-circuit immediately**: return empty slice BEFORE `freeTextIDsForType`, `applyV1Filters`, or `applyV1Sorting` runs. Search backend MUST NOT be invoked. Pin with a mock-searcher regression test. Addresses RR-X56H.
  - `Query` → `st.GraphQuery(ctx, *rqr.Query)` collect; subsequent search/filter/sort runs on the filtered slice (intersection-after-ACL — pin call ordering via mock; RR-WX77 + RR-3IO2).
- **`internal/dataentry/api_v1.go` — `handleV1ListEntities` pagination headers** (line ~462). Verify all leak surfaces reflect post-filter count: `data.length`, `meta.total`, `meta.has_more`, `X-Total-Count`, `X-Page`, `X-Per-Page`, **`Link rel="next"`** (must not appear if no visible next page exists even when hidden pages exist after), `Link rel="last"`. Pin each in AC3. Addresses RR-KNGC + RR-VDTW.
- **`internal/dataentry/api_v1.go` — per-principal caching headers.** List responses are now per-principal. Set `Cache-Control: private, no-store` (or rely on existing `noCacheMiddleware` — verify it's sufficient) and add `Vary` on whatever header carries the principal stamp (the `--principal-header` flag value). Prevents downstream CDN/proxy/browser cache from leaking principal A's response to principal B. Addresses RR-VDTW.
- **`internal/dataentry/api_v1.go` — DenyAll list `_actions`.** Pin in AC4: `_actions.create == false` on DenyAll list response (no Create button surfaces for a denied principal). Backed by the policy-load invariant in the next bullet. Addresses RR-FF7Q + RR-W2J6.
- **`internal/acl/policy.go` — policy loader rejects "read-deny + write-grant" roles at startup.** A role with `write: [ticket]` but no `read: [ticket]` (or matching wildcard) is rejected at boot with a structured error. Prevents the data:[] + create:true UX nonsense and lets all downstream affordance logic assume "if you can write a type, you can read it." Test with a malformed policy fixture. Addresses RR-W2J6.

### Sidebar (single-mode collapse)

- **`internal/dataentry/api_v1.go` — `sidebarCounts` collapsed to one path**: always go through `gate.Query(type)`, regardless of NopACL vs. ACL. **Drop the precomputed `typeCounts` cache entirely** — `GraphCount(GraphQuery{EntityType: type})` is identical cost to today's `Store.CountEntities(type)`. `filterCache` (within-request memo, keyed by listID) stays. Addresses RR-BZ4M + RR-2O27.
- Both `listCount` and `kanbanCount`:
  - No filters → `GraphCount(rqr.Query)` returns matched count directly.
  - With filters → `GraphQuery(rqr.Query)` to get visible entities, then `applyFilters` in-memory, then `len`. Ordering: ACL → config filter → count.
- **Sidebar config-filter perf caveat documented** (no code change): "evaluates in-memory after ACL GraphQuery. Performance scales with visible-set size. For visible sets >10k, prefer pre-filtering via entity_type in nav config or file a follow-up to push filters into GraphQuery." Addresses RR-REQW.
- **Sidebar menu structure** stays principal-independent (reveals metamodel shape, not data shape). Documented in `GUIDE-acl-security`; future ticket can tighten. Addresses RR-KNGC (menu half).

### Docs

- Full rewrite of the "Read-path gating" section of `GUIDE-acl-security` to describe both PRs together (per-entity from TKT-VQGN + aggregates here), the menu-visibility decision, the search-ordering contract, the policy-load invariant, the config-filter perf caveat.
- Document `_position` as still deferred; point to the follow-up ticket.

## Out of scope (separate future tickets)

- **`/api/v1/_position` filtering** — list-derived ordinal computation that takes an id; deferred until after PR 2 lands so this PR stays focused on list response shape. Open RRs RR-NDMN, RR-37IY, RR-ATSO carry over to that ticket.
- `/_search` ACL filtering (use the search-after-ACL ordering pinned in AC9 as the contract).
- SSE `/api/v1/_events` per-subscriber visibility.
- MCP transport intersection (TKT-G3PPD).
- Property-level redaction on read.

## Acceptance criteria

1. **Type-level read grant on list.** Role R with `read: [ticket]` sees every ticket via GET `/api/v1/tickets`; sees `data: []` for any type without a grant.
2. **Role-relation grant with inheritance on list.** `editor-of` confers `editor` (read on ticket), `inherit_roles_through: [belongs-to]`, P linked to PRJ-42, TKT-001 belongs-to PRJ-42, TKT-002 belongs-to PRJ-9: `GET /api/v1/tickets` returns only TKT-001.
3. **All eight pagination/header leak surfaces reflect post-filter count.** For 5 visible + 5 hidden, `per_page=3`, `page=2`: `data.length == 2`, `meta.total == 5`, `meta.has_more == false`, `X-Total-Count: 5`, `X-Page: 2`, `X-Per-Page: 3`, `Link rel="last"` → `page=2`, `Link rel="next"` absent. None mention 10.
4. **DenyAll list shape + `_actions`.** No-read principal hitting `/api/v1/tickets` gets `data: []`, `meta.total: 0`, all pagination headers 0, **`_actions.create == false`**. Backed by AC8 policy-load invariant (read-deny + write-grant roles rejected at boot).
5. **DenyAll search short-circuit.** Mock-searcher regression test: DenyAll list path does NOT invoke `freeTextIDsForType` / `applyV1Filters` / `applyV1Sorting`. Asserts on call counts.
6. **Sidebar `listCount` matches list.** For a principal who sees 5 of 10 tickets, sidebar list-item `count == 5`. Same for `kanbanCount`.
7. **Sidebar count under config filter.** A list with `filters: [{property: status, equals: open}]`, 10 total (5 open + 5 closed), principal sees 3 of those 10: sidebar count reflects intersection (visible-and-open). Single-mode path verified — no separate "ACL on/off" code branch.
8. **Policy-load rejects read-deny + write-grant.** Malformed policy fixture with role having `write: [ticket]` but no `read: [ticket]` fails `appbuild.loadACLPolicy` with a structured error.
9. **Search-after-ACL ordering pinned (mock-asserted).** Mock searcher records call order; test asserts searcher invoked AFTER `GraphQuery`, on the filtered slice only.
10. **Per-principal caching headers.** List responses carry `Cache-Control: private, no-store` (or equivalent via existing middleware) and `Vary: <principal-header>`. Two-principal test: alice's cached response is not served to bob.
11. **NopACL regression.** Without `acl.yaml`, list / sidebar responses are structurally identical to today (JSON-canonical compare via the PR-1-shared helper).
12. **`GUIDE-acl-security` updated** to describe both gates, menu-visibility decision, search-ordering contract, config-filter perf caveat, and the `_position` deferral pointer.

## Files to modify

- `internal/dataentry/api_v1.go` — `scopedSortedEntities`, `handleV1ListEntities` pagination/headers, `_actions`, `sidebarCounts` single-mode.
- `internal/acl/policy.go` — read-deny + write-grant rejection at load.
- `internal/acl/policy_test.go` — policy-load rejection test.
- `internal/dataentry/acl_list_test.go` (new) — AC1, AC2, AC3, AC4, AC5, AC9, AC10.
- `internal/dataentry/acl_sidebar_test.go` (new) — AC6, AC7.
- `internal/dataentry/acl_list_regression_test.go` (new) — AC11 NopACL.
- `docs-project/entities/guides/GUIDE-acl-security.md` — full read-path rewrite.
- `docs/acl-security.md` — regenerated.

## Stack

Stacks on TKT-VQGN's PR. CI blocked until the full chain (903 → 905 → 910 → 911
→ TKT-VQGN PR) lands in develop. Stacking on PR 1 (rather than PR 911 directly)
is a deliberate rebase-minimization choice — PR 1 touches shared files this PR
also modifies.

## Follow-ups to file when this lands

- **`ACL read-side: /api/v1/_position filter + per-id gate`** (list-derived; deferred from both PRs to keep scope tight; carries RR-NDMN, RR-37IY, RR-ATSO).
- `ACL read-side: /_search filtering for bleve and pgstore backends` (use search-after-ACL contract pinned by AC9).
- `ACL read-side: SSE event broker per-subscriber visibility`.
- `ACL read-side: sidebar config filters push down into GraphQuery` (if visible-set sizes warrant).
