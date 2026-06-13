---
id: PLAN-ELIA7T
type: planning-checklist
title: 'Planning: ACL read-side: /_search visibility — VisibleSearcher seam, generic + pgstore-native impls, conformance suite'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: `search.VisibleSearcher` seam + neutral `TypeScope` type with
exact→`"*"`→DenyAll lookup rule; generic hit-filter implementation
(`search.NewVisible`: backend Limit:0 candidates → MatchingIDs batch filter →
post-filter truncation to Query.Limit); pgstore-native `(*Store).SearchVisible`
combined-SQL implementation; `storetest.RunVisibleSearchTests` conformance suite
(cases 1–8, per-backend invariants, 3 backends); gate internalized in
`executeQuery` (error return preserving errACLListQuery/errListLoad taxonomy);
`handleV1Search` error mapping + includeRelations=false pin; `resolveScope`
retires `readableSubset`; appbuild VisibleSearcher slot per recipe; docs.

OUT: SSE per-subscriber visibility; `_position` per-id gate
(RR-NDMN/RR-37IY/RR-ATSO); legacy `Backend.Search` ctx threading;
property-filter SQL pushdown; sidebar config-filter pushdown;
`listFromStoreByTypes` error refactor for its other 3 callers (RR-FCVX69).

**Acceptance Criteria:** AC1–AC10 (rev 2, incl. 3b/7b) in TKT-BA8BSX body, each
with concrete test scenario (see Test Plan below).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: design settled in a full interactive discussion 2026-06-12 covering options, tradeoffs, and recommendation — the ticket body IS the research record; spinning a RES would duplicate it)
- [x] ~~Searched for existing libraries~~ (N/A: in-tree seam; no external library applies to per-principal graph-ACL search filtering)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (ticket body carries the full design rationale +
discussion provenance)

**Existing Solutions:**

- `storetest` conformance pattern: `internal/store/storetest/storetest.go` (Factory/SearchFactory, RunAll, nil-skippable suites) — directly extended, not reinvented.
- "Smart backend" split anticipated by `search.Searcher` godoc (`internal/search/types.go:17`) — this ticket is the first instance of that promise.
- Hit-filter batching: `readableSubset` (`internal/dataentry/scope.go:130`) from TKT-VMD8 — same shape, moved to hits-before-body-load.
- SQL composition: `buildGraphQuerySQL`/`buildPredicateSQL`/`buildMatchingIDsSQL` (`internal/store/pgstore/graphquery.go`) — reused in-package for the EXISTS chains; per-type CTE prefixing is new.
- Search-after-ACL ordering contract: TKT-VMD8 AC9 + `TestACLList_SearchAfterACLOrdering`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Full design in TKT-BA8BSX body §Design (rev 2). Summary:
consumer (dataentry) resolves per-type `ReadQuery` verdicts over `meta.Entities`
into `map[string]search.TypeScope` (NopACL → `{"*": AllowAll}`); `SearchVisible`
executes the scope. Generic impl filters hits via `MatchingIDs` (candidates at
backend Limit:0, bleve 10k floor accepted), truncates post-filter. pgstore impl
is a method on `*pgstore.Store` composing visibility into the trgm statement
(per-type disjunction, LIMIT post-visibility, ctx threaded). Cap model: 1000
stays the final /_search bound, applied post-visibility (NopACL byte-parity
preserved). Conformance suite with scope literals pins all implementations.

**Alternatives rejected:**
- Handler-level post-filter (patch `handleV1Search` with `readableSubset`): leaves `executeQuery` ungated for future consumers, keeps cap starvation, loads hidden bodies. Rejected — closes the hole, not the class.
- `search → acl` import (pass `acl.ReadQueryResult` directly): new arch edge for no benefit; neutral `TypeScope` mirrors it with store types search already imports.
- Optional capability interface + type-assert at consumer: violates the no-back-channel rule; recipe wiring is explicit and fits the documented appbuild pattern.
- bleve doc-ID filter pushdown: requires building the allowed-ID set per request anyway, defeating the point; local MatchingIDs is already cheap.
- Exporting pgstore SQL builders for an out-of-package native impl: rejected in favor of a method on `*pgstore.Store` (RR-1LFQA5).

**Files to modify:** listed in TKT-BA8BSX §Files (rev 2).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `q` query param (user-controlled free text): parsed by existing `searchparser.ParseQuery`; reaches SQL only via `sqlBuilder.arg` positional placeholders + `escapeLike` (existing, reused). No new injection surface.
- `type` query param: validated against metamodel (existing behavior).
- Scope map: server-derived from policy, never wire-supplied. Fail-closed by construction: exact → `"*"` → DenyAll; the dangerous failure mode (nil map) denies everything rather than allowing it.

**Security-Sensitive Operations:**
- Visibility filtering (the point of the ticket): deny-by-default lookup; conformance no-leak invariant on every case; serializer-level leak (AC3b) covered at the dataentry layer since the seam suite can't see it (RR-QO01XY).
- Error responses: constant detail + slog.Warn per POLICY-015; AC7 asserts synthetic error strings absent from bodies; AC7b preserves the canceled→silent / deadline→504 mapping.
- Timing side channel: all-effective-DenyAll short-circuits before backend (AC4); pgstore-native does no hidden-row work; generic path's residual local timing signal accepted + documented.
- DoS bound (RR-BDYTP5): candidate set capped per backend; MatchingIDs batched per type over the bounded set.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1/AC2/AC3/AC3b: `acl_search_test.go` HTTP-level tests reusing VMD8 world seeders + raw-body leak assertions (incl. visible-hit-related-to-hidden-entity case; includeRelations=false pin).
- AC4: recording-searcher — zero calls under all-DenyAll, exactly one under single-allowed-type.
- AC5/AC6: `storetest.RunVisibleSearchTests` ×3 (generic+memstore/linear, generic+fsstore/bleve in search package tests; pgstore-native in conformance_test.go, DB-gated, `just test-postgres`).
- AC7/AC7b: failing GraphQueryer + failing searcher fakes → 5xx constant detail, body free of synthetic string; `errors.Is` asserts on sentinel wrapping; canceled/deadline mapping on `_position?scope=search`.
- AC8: existing `TestACLPosition_SearchScopeGated` unchanged and green.
- AC9: NopACL JSON-canonical compare on `/_search`.
- AC10: docs review + `just docs` regeneration.

**Edge Cases:**
- Nil/empty scope map → deny everything, no backend call.
- Empty `q` → existing early-return preserved.
- Hit type absent from scope and no `"*"` → dropped (fail-closed, conformance case 8).
- Limit smaller than visible count with hidden entries ranked above → conformance case 5.
- `Query.Types` requesting a DenyAll type → empty for that type, others unaffected (case 7).
- Type in scope but absent from corpus → no hits, no error.
- Stale index hit (deleted between search and store read): existing skip-silently behavior preserved.
- One type's MatchingIDs errors in a mixed-type result → whole request 5xx (no partial leak-prone response).

**Negative Tests:** AC4 (short-circuit), AC7/7b (error mapping + non-leak),
conformance cases 2 and 8.

**Integration approach:** conformance suite is store+search integration by
construction; dataentry tests are full HTTP handler integration; pgstore path
runs against a real database via `just test-postgres`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- 3-deep stack rework (939 re-review may force cascading rebases): **accepted explicitly by user 2026-06-12**; mitigated by new-file-dominant diff.
- appbuild plumbing for the VisibleSearcher slot touches all three recipes + cmd wiring: contained by the recipe pattern; arch-lint pins boundaries.
- pgstore plan quality (recursive CTE fences + trgm): precedent in `graphquery_explain_test.go` / `graphquery_bench_test.go`; conformance catches correctness.
- Behavioral drift generic vs native: conformance suite + per-backend invariants ARE the mitigation; differential fuzz is the stretch backstop.
- `executeQuery` signature change ripples: exactly two consumers, both updated in this PR; `listFromStoreByTypes` untouched (RR-FCVX69).

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] GUIDE-acl-security (docs-project) — `_search` deferred→gated, seam + scope-lookup rule, post-visibility limit contract, off-metamodel fail-closed note, generic-vs-native split, bleve 10k caveat
- [x] docs/acl-security.md — regenerated via `just docs`
- [x] storetest godoc — new implementations must pass RunVisibleSearchTests
- [x] ~~docs/metamodel.md, cli-reference.md, data-entry.md, CLAUDE.md, README.md~~ (N/A: no metamodel/CLI/UI surface change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 13 findings via cranky-code-reviewer (2026-06-12),
all `addressed` in plan rev 2: RR-1LFQA5 (critical, pgstore wiring), RR-QO01XY
(critical, serializer leak layer), RR-SOU82P, RR-FCVX69, RR-Z65PWC, RR-ODIXVI,
RR-BDYTP5, RR-DLFCHR (significant), RR-AJ0QD8, RR-1R9X50, RR-2LGO3L, RR-X1A3FW,
RR-599CLE (minor).
