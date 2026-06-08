---
id: TKT-VQGN
type: ticket
title: 'ACL read-side (PR 1/2): per-entity GET + writes + ?include= gated; middleware fail-loud; ETag'
kind: enhancement
priority: medium
effort: m
status: done
---

First of a two-PR read-side ACL series. Closes the per-entity-response
enumeration channels (URL probing, write probing, include-walk) without touching
list-style endpoints. Those land in TKT-VMD8 (list / sidebar) and a follow-up
ticket (_position). Each PR has a single, reviewable security surface.

## Why this split

The original TKT-VQGN scoped list + GET + sidebar + _position in one PR. Two
rounds of design review (12 + 19 findings across two reviewers) surfaced 7
criticals. Splitting on the natural seam — per-entity-response chokepoints vs.
list-derived endpoints — drops the per-PR risk and lets each ship a coherent
deny model.

This PR's narrow goal: **anything that returns an entity body (or a derivative
of one) returns 404 if the principal can't see it.** GET, PATCH, DELETE,
POST-action, and `?include=` walks all collapse to the same shape. `_position`
is list-derived (it takes an `id` but the answer depends on a scope walk), so
it's grouped with the other list-style endpoints in a follow-up ticket.

## Scope (this ticket / single PR)

### Store

- **`internal/store/graphquery.go` — add `WhereIDs []string` to `GraphQuery`** as a **predicate** (constrains matched, does NOT affect `total`). Naive impl: `if len(WhereIDs) > 0 && !slices.Contains(WhereIDs, e.ID) continue`. pgstore impl: `AND id = ANY($N)`. Godoc explicitly pins the semantics. Storetest case: 2-entity store, `WhereIDs=[nonexistent]` → `matched=0, total=2`. Addresses RR-8AAH + RR-R0NR.

### ACL

- **`internal/acl/declarative.go` — `Declarative` gains a `graphQueryer store.GraphQueryer` field, set at `NewDeclarative` construction time** (parallel to existing `graph Graph` field). Validated non-nil. Addresses RR-Z4AP.
- **`internal/acl/request.go` — add `Request.Visible(ctx, entityType, entityID) (bool, error)`** implemented via the `readQuery` core:
  - `AllowAll` → `true, nil`
  - `DenyAll` → `false, nil`
  - `Query` → `matched, _, err := d.graphQueryer.GraphCount(ctx, *q WITH WhereIDs=[id]); return matched > 0, err`

Single source of truth with `ReadQuery` — no divergence risk. Feature test
covers AllowAll / DenyAll / role-relation-with-ancestor /
role-relation-no-ancestor / type-not-in-policy.

### Dataentry

- **`internal/dataentry/readgate.go` (new) — consumer-side `readGate` interface** in the dataentry package:
  ```go
  type readGate interface {
      Visible(ctx context.Context, entityType, entityID string) (bool, error)
      Query(ctx context.Context, entityType string) acl.ReadQueryResult
  }
  ```
Production impl wraps `*acl.Request`; `nopReadGate` for the NopACL path.
Handlers pull via `readGateFromContext(ctx)` typed key. All ACL-aware read paths
in this PR use this interface; PR 2's additions inherit the same pattern.
Addresses RR-CB8Y + RR-GR3C.
- **`internal/dataentry/router.go` — `attachACLRequest` fail-loud, scoped to `/api/`** (RR-T15E). When ACL is configured AND `ForPrincipal` errors, return 500 with a structured error. Apply the middleware to the `/api/` mux only, not the outer router that includes the SPA shell and static assets. A misconfigured stamper must NOT make `GET /` or `GET /assets/*.js` return JSON 500. Regression test: ACL configured + misconfigured stamper + `GET /` → 200 SPA HTML; `GET /api/v1/tickets` → 500 acl_unstamped_principal. Addresses RR-875A + RR-T15E.
- **`internal/dataentry/api_v1.go` — gate `handleV1GetEntity`** (line ~722):
  - After `getEntity` lookup, before the existing `If-None-Match` branch: probe `gate.Visible(...)`.
  - Not visible → 404 with the exact same body the existing not-found path emits.
  - On deny: **do NOT compute or emit `ETag`**; do NOT honor `If-None-Match` (always 404, never 304). `Cache-Control` is already strict via `noCacheMiddleware`; rely on it rather than redundantly setting `private, no-store`. Addresses RR-MZU4 + RR-PLEQ.
- **`internal/dataentry/api_v1.go` — gate `resolveV1Includes`** (line ~1677): **batched by neighbour-type**. Collect candidate target IDs grouped by type, then for each type call `gate.Query(type)` once — `AllowAll` accept all of that type; `DenyAll` drop all; `Query` run ONE `GraphQuery(WhereIDs=[ids…])` per type and intersect IDs in Go. Cuts the worst case from O(N) round-trips to O(distinct-types) per include resolution. Addresses RR-M84L + RR-FRK1.
- **`internal/dataentry/handlers_api.go` — gate write paths via `gate.Visible`** at handler entry, **BEFORE body parse, BEFORE `If-Match`, BEFORE `IsLocked`**. Applies to `handleV1UpdateEntity`, `handleV1DeleteEntity`, and `handleV1EntityAction`. Hidden target → 404 with same body as GET-on-nonexistent. Visible-but-write-denied still hits today's `writeForbiddenIfACLDenied` (403 with `rule_id`). Addresses RR-3532 + RR-FGUZ.

### Error handling

- `Visible` error mapping: `errors.Is(err, context.Canceled)` → emit nothing (client disconnected); `errors.Is(err, context.DeadlineExceeded)` → 504; else 500 with `acl_query_failed` code. Addresses RR-89XK.

### Docs

- New section in `GUIDE-acl-security` describing the per-entity-response gate shape (404, no ETag, no body distinction, includes filter, write parity).
- Pin in `GUIDE-acl-security`: "the method dispatcher MUST NOT consult entity existence — URL-shape only; per-method handler probes `Visible`." Test backs it. RR-V49F.
- Pin in `GUIDE-acl-security`: "all conditional-request headers (`If-None-Match`, `If-Modified-Since`, `If-Match`, `If-Unmodified-Since`, `If-Range`) MUST short-circuit on deny before consulting entity state." RR-ESHJ.
- Update the "Read-path gating" section pointer to "per-entity-response gated; list/sidebar/_position land in follow-up tickets."

## Out of scope (TKT-VMD8 — PR 2/2)

- `handleV1ListEntities` / `scopedSortedEntities` filtering.
- Sidebar `listCount` / `kanbanCount` gating.
- Pagination headers (`X-Total-Count`, `Link`, `X-Page`).
- `_actions.create` on empty/DenyAll list response.

## Out of scope (separate future tickets)

- `/api/v1/_position` filtering — list-derived; file follow-up after PR 2 lands.
- `/_search` ACL filtering.
- SSE `/api/v1/_events` per-subscriber visibility.
- MCP transport intersection (already TKT-G3PPD).
- Property-level redaction on read (FEAT-AESD4 future).

## Acceptance criteria

1. **Type-level read grant on per-entity GET.** Role R with `read: [ticket]` → GET ticket 200; GET document 404. 404 body structural-equal (JSON-canonical, documented volatile-key allowlist) to a 404 from a nonexistent ID.
2. **Role-relation grant with inheritance on per-entity GET.** `editor-of` confers `editor` (read on ticket), `inherit_roles_through: [belongs-to]`, P linked to PRJ-42, TKT-001 belongs-to PRJ-42: GET TKT-001 → 200; GET TKT-002 (belongs-to PRJ-9) → 404.
3. **Write paths 404 on hidden target, BEFORE any other validation.** PATCH/DELETE/POST-action on hidden entity returns 404. **PATCH on hidden with malformed JSON body returns 404, NOT 400.** PATCH on hidden with stale `If-Match` returns 404, NOT 412. AuthorizeWrite 403 only fires on visible-but-write-denied.
4. **Include filter (batched).** `GET /api/v1/projects/PRJ-1?include=*` returns `included` containing only entities the principal can see; hidden neighbours silently omitted. Variant: `include=tickets` with zero `read: [ticket]` returns empty `included`. **Perf:** the include resolver uses one `GraphQuery` per distinct neighbour-type, not one per neighbour.
5. **ETag fully suppressed on deny.** Denied per-entity GET response **carries no `ETag` header** (absence test), and `If-None-Match: <alice-etag>` from another principal on the same URL returns 404, NOT 304.
6. **NopACL regression.** Without `acl.yaml`, GET / PATCH / DELETE / POST-action / include responses are structurally identical to today (JSON-canonical compare, drop volatile keys).
7. **Middleware fail-loud, scoped to `/api/`.** ACL configured + unstamped principal: `GET /api/v1/tickets` → 500 `acl_unstamped_principal`; `GET /` → 200 SPA HTML; `GET /assets/app.js` → 200 (no ACL fail-loud on SPA / static assets).
8. **GraphCount error mapping.** Context-canceled → no response (client gone); deadline-exceeded → 504; other store errors → 500.
9. **`GUIDE-acl-security` updated** to describe the per-entity-response deny model (GET, write, include), method-dispatch invariant, conditional-header rule; point to TKT-VMD8 for list/sidebar and the future _position ticket.

## Files to modify

- `internal/store/graphquery.go` — `WhereIDs []string` + predicate-vs-total godoc.
- `internal/store/graphquerynaive/naive.go` — WhereIDs filter.
- `internal/store/pgstore/graphquery.go` — `AND id = ANY($N)`.
- `internal/store/storetest/graphquery_test.go` — WhereIDs conformance + total semantics.
- `internal/acl/declarative.go` — `graphQueryer` field on `Declarative`, `NewDeclarative` parameter.
- `internal/acl/request.go` — `Visible`.
- `internal/acl/readquery.go` — factor predicate composition so `Visible` and `readQuery` share it.
- `internal/acl/features_test.go` — `Visible` feature tests.
- `internal/appbuild/appbuild.go` — pass store as `GraphQueryer` to `NewDeclarative`.
- `internal/dataentry/readgate.go` (new) — consumer-side interface + adapters.
- `internal/dataentry/router.go` — `attachACLRequest` fail-loud, scoped to `/api/`.
- `internal/dataentry/router_test.go` — fail-loud regression + SPA-reachable test.
- `internal/dataentry/api_v1.go` — `handleV1GetEntity` gate, ETag suppression, `resolveV1Includes` batched gate.
- `internal/dataentry/handlers_api.go` — write-path Visible probe (before parse/IsLocked/If-Match).
- `internal/dataentry/acl_get_test.go` (new) — AC1, AC2, AC4, AC5.
- `internal/dataentry/acl_write_test.go` (new) — AC3.
- `internal/dataentry/acl_regression_test.go` (new) — AC6 NopACL.
- `docs-project/entities/guides/GUIDE-acl-security.md` — per-entity section + invariants.
- `docs/acl-security.md` — regenerated via `just docs`.

## Stack

Stacks on PR 911 (`feat/acl-v1-wiring`). CI blocked until 903 → 905 → 910 → 911
land in develop. TKT-VMD8 stacks on this PR.

## Effort

m. Roughly 300-400 LOC including new test files; store change is +1 field + 2
backend impls.

## Follow-up to file when this lands

- `ACL read-side: /api/v1/_position scope-list filter + per-id gate` (list-derived; deferred from PR 1 since it computes from a scope walk).
