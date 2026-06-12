---
id: TKT-BA8BSX
type: ticket
title: 'ACL read-side: /_search visibility — VisibleSearcher seam, generic + pgstore-native impls, conformance suite'
kind: enhancement
priority: high
effort: m
status: done
---

Follow-up named by TKT-VMD8 ("/_search ACL filtering for bleve and pgstore
backends"). Stacks on TKT-VMD8's PR 949 (accepted rework risk: 3-deep stack
while PR 939 awaits re-review). Design-reviewed 2026-06-12: 13 findings
(RR-1LFQA5 … RR-599CLE), all addressed in this revision.

## Problem

After TKT-VQGN + TKT-VMD8, every read surface is gated except `/api/v1/_search`.
`handleV1Search` calls `executeQuery` with zero gate interaction — a denied
principal can free-text search and receive **full serialized entity bodies**.
The `_position` search scope already patches this externally via
`readableSubset`; the search view itself is the deferred hole (named in
`scope.go` comment).

## Design (rev 2, post design-review)

### Seam

New `search.VisibleSearcher` interface + neutral scope type (no `search → acl`
import edge):

```go
type TypeScope struct {
    AllowAll bool
    Query    *store.GraphQuery
}
// Scope lookup rule (RR-SOU82P / RR-2LGO3L): exact type entry → else the
// reserved "*" wildcard entry → else DenyAll (fail-closed).
type VisibleSearcher interface {
    SearchVisible(ctx context.Context, q Query, scope map[string]TypeScope) iter.Seq2[Hit, error]
}
```

- `Query.Limit` contract: max **visible** hits returned — applied post-visibility (this is the seam-level cap-starvation fix; conformance case 5 exercises it directly with a small explicit limit).
- dataentry builds the scope from `readGate.ReadQuery` over **metamodel types** (`meta.Entities`); `nopReadGate` → `{"*": {AllowAll: true}}` (preserves today's visibility of off-metamodel entities → AC9 safe). Under ACL, no `"*"` entry is emitted, so off-metamodel types (permissive storage: removed/typo types) fail **closed** — documented behavior, pinned by conformance case 8.
- ACL resolution stays at the consumer; `SearchVisible` only executes a scope.

### Generic implementation (bleve + linear)

`search.NewVisible(searcher Searcher, gq store.GraphQueryer)` wrapping the
existing service: fetch candidates with backend `Limit: 0` (**note RR-AJ0QD8:
bleve maps ≤0 to a 10000 floor — accepted and documented**: cap starvation is
fixed within a 10k-candidate window on bleve, fully on linear/pgstore), group
hits by type, batch via `MatchingIDs`, truncate to `Query.Limit` **after**
filtering. Local backends pair only with fsstore/memstore, so `MatchingIDs` is
in-process (graphquerynaive); candidate set bounded by the backend (10k bleve /
corpus linear) bounds the DoS surface (RR-BDYTP5).

### pgstore-native implementation (second commit)

**Method on `*pgstore.Store`** (RR-1LFQA5 resolution — in-package access to
`buildPredicateSQL`/`escapeLike`/`sqlBuilder`; no export of SQL internals): `(s
*Store) SearchVisible(ctx, q, scope)` builds one statement — trgm `LIKE` ANDed
with per-type visibility disjunction (bare `e.type=$n` for AllowAll incl. `"*"`
handling as no type-restriction clause, EXISTS chains for Query verdicts with
**per-type CTE prefixes**, DenyAll omitted), `ORDER BY similarity` then `LIMIT`
**post-visibility**. ctx threads into `db.Query(ctx, …)` (RR-1R9X50 — new code
is cancellable; the legacy `Backend.Search` background-ctx wart stays tracked
separately).

### Wiring (RR-1LFQA5)

New collaborator slot threaded `appbuild` → cmd wiring → `dataentry.App`: fs +
memory recipes wire `search.NewVisible(searcher, st)`; postgres recipe wires the
`*pgstore.Store` itself (it already implements `GraphQueryer`; now also
`VisibleSearcher`). `dataentry.NewApp` validates non-nil per the constructor
rule. No build-agnostic code branches on backend.

### Cap model (RR-ODIXVI / RR-Z65PWC resolution — dissolves the AC6↔AC9 contradiction)

`maxFreeTextSearchResults = 1000` **stays** as the final `/_search` result
bound, but moves from pre-ACL candidate cap to post-visibility truncation
(dataentry passes `Limit: 1000` to `SearchVisible`). All-AllowAll / NopACL:
filter is a no-op and the ranker is unchanged, so the top-1000 is byte-identical
to today → NopACL JSON-canonical regression holds. Restricted principals:
top-1000 of **their** visible corpus (within backend candidate bounds).
Non-free-text queries (`type:x` only): unbounded today, stays unbounded (matches
list-endpoint semantics), now gated.

### Gate placement

Inside `executeQuery`: signature gains `error`. Scope construction happens
first; **all-effective-DenyAll short-circuits after scope construction, before
any backend call** (RR-599CLE: decision point pinned; positive case also pinned
— one allowed type ⇒ exactly one searcher invocation). Error wrapping preserves
the existing sentinel taxonomy (RR-DLFCHR): gate/scope errors wrap
`errACLListQuery`, load errors wrap `errListLoad`, so
`writeListPipelineError`/`writeGateError` mappings (canceled→silent,
deadline→504, constant detail + slog.Warn per POLICY-015) fire identically for
both consumers. `resolveScope` retires `readableSubset`.

**`listFromStoreByTypes` is NOT refactored** (RR-FCVX69: 4 callers — app.go:519,
commands.go:322, handlers_api.go:861, helpers.go:406). executeQuery's
non-free-text branch gets its own inline error-surfacing iteration (mirrors the
VMD8 AllowAll fix); the other three callers are untouched and out of scope.

### Serializer leak surface (RR-QO01XY / RR-X1A3FW)

The hit filter governs root-entity survival only. `handleV1Search` keeps
`includeRelations=false` — now pinned by comment + test, not convention. New
dataentry-level AC: a **visible** hit that relates to a **hidden** entity
exposes no hidden ID/title/property through any field of its serialized body
(raw-body assertion). AC1/AC3 verification is explicitly assigned to dataentry
handler tests against the real `handleV1Search`; the storetest conformance suite
covers the seam (Hit-level) only.

### Limit vs Go-side property filters

When `sq.PropertyFilters` remain after the search, the 1000 truncation applies
after them (no early truncation) — starvation must not sneak back through the
filter gap.

### Conformance suite

`storetest.RunVisibleSearchTests` + `VisibleSearchFactory func(t) (store.Store,
search.VisibleSearcher)`, next to `SearchFactory`/`RunSearchTests`. Scope
**literals**, not acl.Policy. Run for generic+memstore/linear,
generic+fsstore/bleve, pgstore-native (DB-gated on RELA_TEST_DATABASE_URL). No
capability flag.

Cases: (1) all-AllowAll parity with ungated Search — **same backend, same Query
incl. limit** (RR-Z65PWC: baseline defined per-backend); (2) DenyAll-by-absence
never hits even on best text match; (3) Query verdict filters by predicate; (4)
mixed AllowAll+Query+DenyAll scope in one query; (5) **limit is
post-visibility** — small explicit `Query.Limit`, top-ranked candidates hidden,
visible below still fill the result; (6) transitive predicate
(InheritThrough/depth) exercising pgstore's recursive-CTE composition; (7)
`Query.Types` ∩ scope composes (incl. Types requesting a DenyAll type); (8)
**wildcard + fail-closed lookup**: `{"*": AllowAll}` admits unknown types; map
without `"*"` denies a type absent from it.

Invariants asserted in every case, **per-backend** (RR-Z65PWC): **no-leak**
(every hit ID ∈ allowed set) and **ordered-subsequence** (visible result ==
same-backend ungated result with hidden entries deleted, order preserved;
fixture corpora stay below all candidate bounds so truncation never confounds it
— truncation itself is pinned separately by case 5).

Out of the suite (dataentry handler tests instead): serializer-level leaks
(AC3/AC3b), DenyAll-skips-backend recording assertions, error-path mapping.

Stretch (droppable to follow-up): differential check — random corpus + random
scope, generic-over-memstore ≡ pgstore visible **sets**.

## Out of scope

- SSE `/api/v1/_events` per-subscriber visibility (separate named follow-up)
- `_position` per-id gate (carries RR-NDMN/RR-37IY/RR-ATSO; separate follow-up)
- `Backend.Search` ctx threading for the legacy method
- Property-filter pushdown into SQL; `listFromStoreByTypes` error refactor for its other 3 callers

## Acceptance criteria (rev 2)

1. **Denied principal gets nothing from /_search.** No read grants: `GET /api/v1/_search?q=<matching text>` → `data: []`, total 0. With per-type grants: only visible entities, full bodies only for those.
2. **Role-relation inheritance on search.** alice/editor-of/PRJ-42 world: search matching TKT-001 (visible) and TKT-002 (hidden) returns only TKT-001.
3. **No-leak on the wire** (dataentry handler test, raw body): hidden IDs/titles/property values absent in AC1/AC2 responses. **3b (RR-QO01XY):** a visible hit relating to a hidden entity exposes no hidden ID/title/property through any serialized field; `includeRelations=false` pinned by test.
4. **Short-circuit + positive control (RR-599CLE).** Recording-searcher: all-effective-DenyAll scope → zero backend calls; single-allowed-type query → exactly one backend call.
5. **Conformance suite green on three implementations** (cases 1–8 + per-backend invariants): generic+memstore/linear, generic+fsstore/bleve, pgstore-native (DB-gated).
6. **Cap starvation fixed at the seam**: conformance case 5; `/_search` passes Limit 1000 post-visibility (bleve 10k candidate floor documented as accepted bound).
7. **Error semantics**: gate/store failure → 5xx constant detail ("check server logs"), raw error only in slog; synthetic error string asserted absent from body. **7b (RR-DLFCHR):** executeQuery preserves `errACLListQuery`/`errListLoad` wrapping — unit-assert `errors.Is`; canceled-context on `/_position?scope=search` stays silent, deadline → 504.
8. **`resolveScope` parity**: TestACLPosition_SearchScopeGated green after `readableSubset` retirement.
9. **NopACL regression**: without acl.yaml, `/_search` response JSON-canonically identical to today (holds by cap model: no-op filter + unchanged ranker + same final 1000 bound; NopACL scope = `{"*": AllowAll}`).
10. **Docs**: GUIDE-acl-security — `_search` deferred→gated, seam + scope-lookup rule, post-visibility limit contract, off-metamodel fail-closed note, generic-vs-native split, bleve 10k caveat; `docs/acl-security.md` regenerated; storetest godoc points new implementations at RunVisibleSearchTests.

## Files (rev 2)

- `internal/search/types.go` — `TypeScope`, `VisibleSearcher`, scope-lookup rule godoc
- `internal/search/visible.go` (new) — `NewVisible` generic implementation
- `internal/store/pgstore/visiblesearch.go` (new) — `(*Store).SearchVisible`; `graphquery.go` — per-type CTE prefix support
- `internal/store/storetest/visiblesearch.go` (new) — conformance suite; `storetest.go` — `VisibleSearchFactory`
- `internal/store/pgstore/conformance_test.go`, search-package tests — wire suite ×3
- `internal/appbuild/appbuild_{fs,memory,postgres}.go` + cmd/mcp wiring — new VisibleSearcher slot, nil-validated at `dataentry.NewApp`
- `internal/dataentry/helpers.go` — `executeQuery` gate + error return + inline non-free-text iteration; `api_v1.go` — `handleV1Search` error mapping + includeRelations pin; `scope.go` — retire `readableSubset`
- `internal/dataentry/acl_search_test.go` (new) — AC1–4, 3b, 7, 7b, 9
- docs as AC10

## Stack

Branch from `feat/acl-listside-tkt-vmd8` (PR 949). Rework risk from the 3-deep
stack accepted explicitly (2026-06-12). When 939/949 merge, rebase/retarget per
the established cherry-pick pattern.
