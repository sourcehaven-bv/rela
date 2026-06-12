---
id: PLAN-CW1N
type: planning-checklist
title: 'Planning: ACL read-side PR 1/2 — per-entity GET + writes + ?include= gated'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:**

ACL v1 (PRs 903 / 905 / 910 / 911) gates writes through
`entitymanager.AuthorizeWrite` and per-entity affordances. The read path is
**not** ACL-enforced. Original TKT-VQGN scoped list + GET + sidebar + _position
in a single PR; design review surfaced 4 critical + 5 significant findings.

This PR rescopes to **per-entity-id chokepoints only** — the natural seam where
the deny model is "404 indistinguishable from not-found." List/sidebar/_position
land in **TKT-VMD8** (PR 2/2), where the deny model is
"empty-or-filtered-with-no-cardinality-leak."

**Scope (IN):**

- `internal/store`: add `WhereIDs []string` to `store.GraphQuery` (single source of truth so `Visible` and `ReadQuery` share predicate composition). RR-8AAH.
- `internal/acl`: `Request.Visible(ctx, type, id) (bool, error)` implemented via `readQuery → GraphCount(WhereIDs=[id])`.
- `internal/dataentry/router.go`: `attachACLRequest` fail-loud (500 + structured error) when ACL is configured and `ForPrincipal` errors. RR-875A.
- `internal/dataentry/api_v1.go`: gate `handleV1GetEntity` (404 + suppress ETag + `Cache-Control: private, no-store` + ignore `If-None-Match`). RR-MZU4.
- `internal/dataentry/api_v1.go`: gate `resolveV1Includes` (filter each neighbour via `Visible`). RR-M84L.
- `internal/dataentry/handlers_api.go`: write paths (PATCH/DELETE/POST-action) probe `Visible` first; return 404 (not 403) on hidden target. RR-3532.
- Docs: per-entity deny-model section in `GUIDE-acl-security`.

**Scope (OUT — TKT-VMD8 / PR 2):**

- `scopedSortedEntities` / `handleV1ListEntities` filtering.
- Sidebar `listCount` / `kanbanCount` gating + cache aliasing fix.
- `/api/v1/_position` filtering + gap-documentation.
- Pagination headers (`X-Total-Count`, `Link`).
- `_actions.create` on DenyAll list.
- Search-after-ACL ordering contract.
- Optional consumer-side `readGate` interface.

**Scope (OUT — separate future tickets):**

- `/_search` ACL filtering.
- SSE `/api/v1/_events` per-subscriber visibility.
- MCP transport intersection (TKT-G3PPD).
- Property-level redaction on read.

**Acceptance Criteria:** (see TKT-VQGN body for the eight ACs in detail)

1. Type-level read grant: GET ticket → 200; GET document → 404 (no read grant).
2. Role-relation grant + inheritance: GET TKT belongs-to granted-project → 200; GET TKT belongs-to other-project → 404.
3. Write paths return 404 on hidden target (PATCH/DELETE/POST-action).
4. `?include=*` returns only visible neighbours.
5. ETag suppressed on deny; `If-None-Match` returns 404 (not 304).
6. NopACL: GET/PATCH/DELETE/include responses structurally identical to today (JSON-canonical compare).
7. Middleware fail-loud: ACL configured + unstamped principal → 500.
8. `GUIDE-acl-security` describes the per-entity deny model and points to TKT-VMD8 for list/sidebar.

## Research

- [x] For larger features: run `/research` → **N/A, mechanical wiring of existing scaffolding**
- [x] Searched for existing libraries
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — design decisions made in TKT-ZYH3 (GraphQuery DSL),
TKT-GV50 (Request.ReadQuery), FEAT-AESD4 (read filtering semantics).

**Existing Solutions:**

- `acl.Request.ReadQuery` + `readquery.go:readQuery` — exported on PR 911 branch.
- `store.GraphQueryer` (`GraphQuery` + `GraphCount`) — `internal/store/graphquery.go`.
- `World.Visible` test fixture (`internal/acl/testutil_test.go:200`) — canonical consumer pattern; basis for production `Visible`.
- `acl.WithRequest` / `acl.FromContext` (`internal/acl/request.go:114,123`) — middleware ships in PR 911.
- Plone / Postgres-RLS / OpenFGA — silent-hide convention for read-deny; 404-vs-403 for write-deny on hidden entities.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:**

Five mechanical changes, in dependency order:

1. **`store.GraphQuery.WhereIDs []string`.** Naive impl: `if len(WhereIDs) > 0 && !slices.Contains(WhereIDs, e.ID) continue` in `graphquerynaive`. pgstore impl: `AND id = ANY($N)` in `buildGraphQuerySQL`. Add one storetest case to lock semantics. (Decision: add to `GraphQuery` not `RelationPredicate` — `WhereIDs` constrains the result set, not the predicate's endpoint matching.)
2. **`acl.Request.Visible(ctx, type, id) (bool, error)`.** Wraps `readQuery`:
   - `AllowAll` → `true, nil`
   - `DenyAll` → `false, nil`
   - `Query` → `matched, _, err := graphQueryer.GraphCount(ctx, *q with WhereIDs=[id]); return matched > 0, err`

This means `Request` needs a `GraphQueryer` dependency. Today
`Declarative.ForPrincipal` doesn't take one — pass via `Request` construction.
Wire in `attachACLRequest` middleware (which already owns Request construction).
3. **Middleware fail-loud.** In `attachACLRequest`:
```go
req, err := d.ForPrincipal(principal.From(ctx))
if err != nil {
    writeV1Error(w, r, 500, "acl_unstamped_principal", "principal not stamped under ACL", err.Error())
    return
}
ctx = acl.WithRequest(ctx, req)
next.ServeHTTP(w, r.WithContext(ctx))
```

500 not panic — per-request misconfig shouldn't take down the process.
4. **`handleV1GetEntity` gate.** After `getEntity`:
```go
if req := acl.FromContext(ctx); req != nil {
    ok, err := req.Visible(ctx, typeName, entityID)
    if err != nil { writeV1Error(500, ...); return }
    if !ok {
        // Suppress ETag, set private no-store, do NOT honor If-None-Match
        writeV1Error(w, r, 404, "not_found", "Entity not found", "")
        return
    }
}
```

Place BEFORE the existing `If-None-Match` branch so a denied principal cannot
receive 304 against an ETag computed for an allowed principal.
5. **`resolveV1Includes` gate.** Filter `for _, target := range candidates: if req.Visible(ctx, target.Type, target.ID) { included[target.ID] = ... }`. Pulls Request via `acl.FromContext`; nil → AllowAll (today's behaviour).
6. **Write-path Visible probe.** At the top of `handleV1UpdateEntity`, `handleV1DeleteEntity`, `handleV1EntityAction` (after entity lookup but before any state mutation): if `req != nil && !Visible(...)`: return same 404 body GET emits. Today's `writeForbiddenIfACLDenied` path remains for the visible-but-write-denied case (it returns the 403 with `rule_id` only for entities the principal can read).

**Alternatives considered:**

- **(rejected) Per-id walk in `Visible` (RR-8AAH option c).** Forks `readQuery` semantics; divergence is a silent security bug. Pinning via test catches today's regression but not tomorrow's.
- **(rejected) Per-principal ETag** instead of suppressing on deny. More code, harder to reason about, easier to get wrong. Suppressing on deny is the conservative move.
- **(rejected) Panic in middleware on unstamped principal.** Per-request misconfig shouldn't take down the process; 500 is the correct shape.
- **(rejected) Pre-filter inside the affordance resolver.** Affordances are about field/relation grants on a known visible entity; visibility is a separate decision the resolver shouldn't be wrapping.

**Files to modify:**

- `internal/store/graphquery.go` — `WhereIDs []string`.
- `internal/store/graphquerynaive/*.go` — naive WhereIDs filter.
- `internal/store/pgstore/graphquery.go` — `AND id = ANY($N)`.
- `internal/store/storetest/graphquery_test.go` — WhereIDs conformance.
- `internal/acl/request.go` — `Visible` + GraphQueryer dependency in Request.
- `internal/acl/declarative.go` — `Declarative.ForPrincipal` wiring (or pass GraphQueryer at Request scope).
- `internal/acl/features_test.go` — `Visible` feature tests.
- `internal/dataentry/router.go` — `attachACLRequest` fail-loud.
- `internal/dataentry/router_test.go` — fail-loud regression test.
- `internal/dataentry/api_v1.go` — `handleV1GetEntity` + ETag suppression + `resolveV1Includes` gate.
- `internal/dataentry/handlers_api.go` — write-path Visible probe.
- `internal/dataentry/acl_get_test.go` (new), `acl_write_test.go` (new), `acl_regression_test.go` (new).
- `docs-project/entities/guides/GUIDE-acl-security.md` — per-entity section.

**Dependencies:** stacks on PR 911. CI blocked until 903 → 905 → 910 → 911 land.

## Security Considerations

- [x] Input sources identified
- [x] Validation approach defined (allowlist)
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak

**Input Sources & Validation:**

- `entityType`, `entityID` from URL — already validated by v1 router.
- Principal from ctx — populated by `attachPrincipal`; under ACL `attachACLRequest` now fails loud on unstamped (no silent fall-through).
- `acl.Policy` — validated at boot in `appbuild.loadACLPolicy` (PR 911).

**Security-Sensitive Operations:**

- **404-body parity** (GET / writes / include): structural-equal JSON-canonical compare against the not-found body. AC1 + AC3 + AC4.
- **ETag**: not emitted on denied GET; `Cache-Control: private, no-store`; `If-None-Match` ignored on deny path. AC5.
- **Order of checks in GET**: visibility BEFORE existing `If-None-Match` branch so a denied principal cannot ride a 304 against an allowed principal's ETag.
- **No new logging of denied IDs** — high-volume 404 log lines on a denial path would themselves be enumeration-friendly.

## Test Plan

- [x] Test scenarios documented for each AC
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

| AC | Scope | Where |
|----|-------|-------|
| AC1 | Type-level read grant on GET | `internal/dataentry/acl_get_test.go` (new), fsstore + tiny acl.yaml |
| AC2 | Role-relation + inheritance on GET | same file, separate test |
| AC3 | Write paths 404 on hidden | `internal/dataentry/acl_write_test.go` (new) |
| AC4 | `?include=*` filter | `acl_get_test.go` — assert `included` keys ⊆ visible set; key-count not a leak |
| AC5 | ETag suppression + If-None-Match 404 | `acl_get_test.go` — two-principal flow |
| AC6 | NopACL structural-equal | `internal/dataentry/acl_regression_test.go` (new) — JSON-canonical compare, drop volatile key allowlist |
| AC7 | Middleware fail-loud | `internal/dataentry/router_test.go` extension |

Plus a `Visible` feature test in `internal/acl/features_test.go` covering:
AllowAll, DenyAll, role-relation-with-ancestor, role-relation-no-ancestor,
type-not-in-policy.

**Edge Cases:**

- Empty type / nonexistent ID + denied principal → 404 (same as visible nonexistent).
- Wildcard `read: ["*"]` → AllowAll for any type.
- Per-entity GET on nonexistent AND denied → 404 (existence check first; visibility check second; both return same body — AC1 / AC6).
- `If-None-Match` set on a denied GET — must return 404, never 304 (AC5).
- `?include=*` to a denied principal — `included` map present but empty; no key leak via count.
- Write to a NONEXISTENT entity vs write to HIDDEN entity — both must return identical 404 (AC3).
- AuthorizeWrite returning ForbiddenError on a VISIBLE entity — must still 403 with `rule_id` (the existing behaviour). Visible-deny ≠ hidden-deny; only the latter shifts to 404.

**Negative Tests:**

- ACL configured + principal unstamped → 500 (AC7), never reaches read handler.
- Malformed acl.yaml → boot fails (PR 911 invariant; no change here).
- GraphCount error during Visible probe → 500 with `acl_query_failed` shape; do NOT silently allow.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated (xs/s/m/l/xl)

| Risk | Mitigation |
|------|------------|
| **Stack depth — TKT-VQGN can't merge until 903 → 905 → 910 → 911 do.** | Same as TKT-YG35; document in PR description. |
| **`WhereIDs` SQL plan in pgstore.** `id = ANY($N)` should hit the PK index but a large id list could overflow the planner. | Bench `WhereIDs=[1]` against today's path via the existing `graphquery_bench_test.go` pattern. Cap WhereIDs at 1 element (only consumer is `Visible`). |
| **`Visible` adds an extra `GraphCount` per per-entity GET.** | Single-row PK lookup with a where clause is O(1) in pgstore; in fsstore/memstore it's bounded by the `member-of × ancestors` walk the resolver already does. Bench. |
| **ETag suppression breaks SPA prev-page cache reuse.** | SPA uses ETag for revalidation; suppressing it on deny means a denied principal can't get a 304. That's the point. NopACL path unchanged. |
| **404 vs 403 body drift over time.** | AC1 + AC3 + AC6 structural-equal tests pin the contract. |
| **Per-request middleware error shape diverges from other 500s.** | Reuse `writeV1Error` with an `acl_unstamped_principal` code consistent with other v1 error codes. |
| **Write-path Visible probe at wrong layer.** | Probe at handler entry, BEFORE entitymanager call. Today's `writeForbiddenIfACLDenied` stays for visible-but-deny case. Tested separately. |

**Effort:** m (~200–300 LOC including 3 new test files; store change is +1 field
+ 2 backend impls).

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `GUIDE-acl-security` — new per-entity section: 404 deny shape, ETag suppression, write-path parity, include filter.
- [x] `GUIDE-acl-security` "Read-path gating is not yet ACL-enforced" → "Per-entity gated; list/sidebar lands in TKT-VMD8."
- [x] `GUIDE-acl-overview` — minor: pointer note that per-entity reads are now gated; sequence diagram still write-focused.
- [x] `docs/acl-security.md`, `docs/acl-overview.md` — regenerated via `just docs`.
- [x] ~~`docs/data-entry.md`~~ (N/A: SPA already handles 404; no new API shape).
- [x] ~~`docs/metamodel.md`, `docs/cli-reference.md`, `CLAUDE.md`~~ (N/A: internal security refactor; no metamodel/CLI/contract changes).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

12 findings on the original (combined) scope. After rescoping:

- **Addressed in TKT-VQGN (PR 1):** RR-MZU4, RR-M84L, RR-3532, RR-875A, RR-8AAH, RR-KBGY.
- **Deferred to TKT-VMD8 (PR 2):** RR-BZ4M, RR-KNGC, RR-NDMN, RR-FF7Q, RR-WX77, RR-CB8Y.

Re-running `/design-review` on the rescoped TKT-VQGN + TKT-VMD8 before
implementation to catch any new issues the split introduces.
