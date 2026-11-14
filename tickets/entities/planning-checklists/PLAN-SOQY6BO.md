---
id: PLAN-SOQY6BO
type: planning-checklist
title: 'Planning: Multi-hop roll-up via'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `via:` takes a list of up to 3 relations (a scalar stays valid);
every function and `where:` apply to the far end; distinct far entities per
source; the reader's row gate at every hop; all four backends; lists, dashboards
and views keep filtering/sorting/grouping on path roll-ups; load-time path
errors. Out: `where:` on intermediate hops (later, as a per-hop filter); `avg`
(split into its own ticket); relation-level `visible:` (TKT-0RBFN0).

**Acceptance Criteria:**
1. `via: [a, b]` loads; `via: a` keeps working unchanged.
2. Count/exists/min/max/sum/latest_by over a path count each far entity once per source (two paths to one measure count once).
3. An intermediate entity the reader cannot read breaks the path; a far entity whose where/pick/value field is hidden is excluded as today.
4. Load refuses an unknown relation, a hop that does not start at the previous hop's type, an intermediate hop with several far types, a content-scoped hop, an empty element, and a path longer than 3.
5. A list page's path roll-up costs the same number of store reads at 10 and 50 rows.
6. pgstore, sqlitestore, memstore and fsstore return the generic reference's answer for path queries.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the approach extends the existing roll-up statement; no competing designs)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-store SQL generation)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: Notion/Airtable roll-ups are single-hop; chaining is done through helper roll-ups)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `predicatefns.ResolveTraversal` already resolves multi-hop `related()` paths with the exact load errors we need (unknown relation, wrong start type, intermediate union, last-hop `type:`).
- `metamodel.StringOrSlice` is the scalar-or-list YAML shape used by `notify: to: via:`.
- `store.RollupQuery`/`RollupGeneric` and the pg/sqlite grouped statements; `storetest.RunRollupTests` compares backend and generic answers.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- metamodel: `RollupDef.Via` becomes `StringOrSlice`; shape check rejects empty elements and more than 3.
- rollup: resolve the path with `ResolveTraversal`; refuse content-scoped hops; `Column.Path` holds the hops before the last.
- store: `RollupQuery.Path []RollupStep{Relation, SourceIsTo, Through GraphQuery}` before the final `Relation`. `RollupGeneric` walks it: per step one relation read and one gated header read. pg/sqlite build a CTE of distinct (source, entity) pairs per hop, joined to the gated rows of the step type, and fold over the final pairs.
- visibility: each step's `Through` is the reader's read query for that type, in the request's world; a DenyAll step yields no value (count 0).
- Max 3 relations: one join per hop in one statement and a multiplicative pair fan-out; the GRC paths need 2, evidence per framework needs 3.

Alternatives rejected: per-intermediate single-hop roll-ups combined in Go
(double counts sum/count, no distinct); falling back to the Go fold on pg/sqlite
(more queries per column than the native statement).

**Files to modify:** internal/metamodel/rollup.go, namespaces.go;
internal/rollup/rollup.go; internal/store/rollup.go;
internal/store/pgstore/rollup.go, graphquery.go;
internal/store/sqlitestore/rollup.go, graphsql.go;
internal/store/storetest/rollup.go; internal/visibility/rollup.go; tests;
docs-project GUIDE-metamodel.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Relation names come from operator-authored
schema.yaml; resolved against the metamodel at load (unknown names refused). All
names reach SQL as bind arguments.

**Security-Sensitive Operations:** Read ACL: each hop's rows are the reader's
own read query (gate before fold). A hidden intermediate breaks the path, so a
count never discloses hidden links. Result is per request, never cached.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1/4: metamodel and rollup compile tests. AC2/6: storetest
path cases (two paths to one far entity, inverse hops, every function, world
faces on intermediates) on every backend vs generic. AC3: visibility test with a
hidden intermediate; dataentry wire test per principal. AC5: Counting budget
test with a path column.

**Edge Cases:** Two intermediates linking to the same far entity; a source with
no intermediates; an intermediate with no far entities; a cycle back to the
source type; a draft face on an intermediate.

**Negative Tests:** Path of 4, empty element, unknown relation, type mismatch,
intermediate union, content-scoped hop.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** SQL drift between backends — mitigated by the generic-reference
comparison in the conformance suite. CTE name clashes in one statement —
mitigated by prefixing per hop. Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md (via docs-project GUIDE-metamodel) - path roll-ups

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Self-review against the design-review checklist; no
critical or significant findings. Minor: the DISTINCT pair CTE also applies to
single-hop queries; acceptable because an identity-scoped edge is unique per
(from, type, to).
