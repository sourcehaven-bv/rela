---
id: PLAN-PW5WNM
type: planning-checklist
title: 'Planning: Roll-up properties: read-time aggregates over related entities'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: a read-only `rollup:` property kind on entity properties (`count`,
`exists`, `min`, `max`, `sum`, `latest_by` + `pick`; one relation `via`,
canonical or inverse; optional `type:` for a union target; `where:` string
equalities). Values are computed at read time with the reader's gate and never
stored. Surfaces: data-entry entity GET (and the write responses that return the
entity), list rows, view sections (and the view entry), MCP `show_entity` (and
`list_entities` refuses filters on roll-ups). Backends: fs, memory, SQLite,
PostgreSQL. Load-time checks; write refusal; refusal of filter/sort on a
roll-up; load error when an automation expression reads one.

Out of scope (backlog tickets): `count_related(...)` predicate, filtering on
`latest_by` values, sorting by a roll-up, roll-up chains, saved roll-ups,
write-time reads (automation conditions/validate, transition guards — guards
cannot read entity properties at all today, statemachine/predicate.go), `avg`,
multi-hop `via`. Lua and search do not see roll-ups.

**Acceptance Criteria:**

1. Schema with the three brief examples loads; a chain (computed reading a
roll-up, roll-up reading a roll-up), unknown relation, unknown property, `pick`
type mismatch, and a write to a roll-up are rejected with clear errors. Test:
`internal/rollup` table tests + metamodel shape tests + entitymanager
create/patch/update refusal tests + data-entry 403.
2. Entity read, list rows and view sections show the right value on every
backend. Test: `storetest.RunRollupTests` (all four backends, pg gated on
`RELA_TEST_DATABASE_URL`) + data-entry handler tests on the default backend.
3. Two principals with different read rights see different values; a hidden
related entity is never counted; a hidden field excludes the row (and an empty
value for a hidden pick). Test: visibility tests with a declarative policy, and
a data-entry end-to-end test with two principals.
4. A list page with N rows and K roll-up columns issues at most K extra store
queries. Test: `storetest.Counting` budget test at 10 and 50 rows.
5. Docs: `docs-project` metamodel source describes roll-ups vs computed.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** ~~RES~~ (N/A: the approved brief G1 is the research outcome;
IDEA-001 Notion-style rollups is the prior art)

**Existing Solutions:**

- Notion/Airtable rollups (count, min/max, latest) are the reference shape.
- `internal/relresolve` (one gated query per traversal per batch) — reused
idea, but it answers existence only.
- `visibility.composeReadScope` (pushdown.go:68) — the reader's row gate for a
type as a store query; the related-row set of a roll-up is exactly this query
for the related type.
- `affordances.PolicyResolver.SearchFieldScope` — the reader's `visible:`
verdicts as store queries; used to exclude rows whose read fields are hidden.
- `pgstore/visiblesearch.go fieldGrantCond` — renders a grant `when:` as
conjuncts on `e`; the roll-up SQL reuses the same builders.
- Gantt handler (TKT-MW28U5, RR-5KEF8E, RR-P751MM): gate before fold.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `metamodel.RollupDef` on `PropertyDef.Rollup`; loader shape checks
(exactly one function, pick⇔latest_by, count→integer, exists→boolean, not with
computed/list/required/default/unique/file, entity-only).
2. New `internal/rollup`: `Compile(meta)` resolves `via` with
`predicatefns.ResolveTraversal` (single hop, no symmetric, no content-scoped),
checks where/value/by/pick properties exist, are not roll-ups, and have
compatible types (min/max: integer/date/datetime; sum: integer; latest_by:
date/datetime; pick: same type as the roll-up). Wired where `computed.Compile`
is (appbuild, schemacheck, projectsetup). `computed.Compile` refuses a
dependency on a roll-up; the automation engine refuses roll-up references in
condition/when/validate; transitions may not be declared on a roll-up property.
3. `store.RollupQuery` + optional `store.Rollupper` with a generic fallback
`store.Rollup` (GraphQueryHeaders + ListRelations + MatchingFaces, fold in Go)
for fs/memory; native single-statement SQL on pgstore and sqlitestore (relations
⋈ the related GraphQuery's source, GROUP BY source). The query carries `Require`
row conditions (field visible) and `PickRequire` (mask). `storetest.Counting`
forwards it as one read.
4. `visibility.Rollups`: per (type, roll-up property) builds the related
GraphQuery from the reader's read scope (deny → empty, allow, graph) in the
reader's world, adds `where` props, derives Require/PickRequire from
`SearchFieldScope(relatedType)`, and calls `store.Rollup` once per column per
batch. Values: count → int, exists → bool, others → value or absent.
5. Surfaces inject values after redaction and skip a roll-up property hidden
on the source row: data-entry GET/write responses/list rows
(`serveListRowExtras`)/view sections (`executeViewBodies` after Filter) and view
entry; MCP `show_entity`. Data-entry field affordances mark roll-ups read-only;
list filter/sort on a roll-up is a 400; entitymanager refuses roll-up keys on
create/patch/update; MCP write pre-check refuses them.

**Alternatives rejected:** materializing (brief: never stored; viewer-
dependent); `relresolve` gate (`GateTraversal`) as the related filter — it
refuses face-restricted, disjunctive and inherited read scopes and every
conditionally visible property, so most roll-ups would fail; Go-side fold on DB
backends — pulls every related row out of the store; withholding the whole
roll-up when a field is conditionally visible — leaks nothing but hides values
the reader could compute.

**Files to modify:** internal/metamodel (types.go, rollup.go, loader.go),
internal/rollup (new), internal/computed, internal/automation,
internal/statemachine or metamodel transition checks, internal/store
(rollup.go), internal/store/{pgstore,sqlitestore}/rollup.go,
internal/store/storetest (rollup.go, counting.go, run lists),
internal/visibility (rollup.go), internal/appbuild (wiring + schemacheck),
internal/projectsetup/validate.go, internal/entitymanager/computed.go,
internal/dataentry (affordances.go, api_v1.go, comment_counts.go, views.go,
views_handler.go, write_handler.go, listfieldgate.go), internal/mcp
(tools_entity.go, tools_helpers.go, tools_schema.go, server deps), docs-project
metamodel doc.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `schema.yaml` (operator config, validated at
load; property names already pass `ValidateSchemaName`, SQL uses bind args or
the builders' safe-literal path). Request: entity ids/page (already gated);
filter/sort names (refused for roll-ups).

**Security-Sensitive Operations:** the aggregate itself. Gate before fold: the
related set is the reader's own read query (row gate, faces, world), so a hidden
related entity never contributes; fields the roll-up reads are required visible
per row through the reader's compiled field scope (the same scope search uses,
pinned to FieldVerdicts); a hidden pick yields empty. The roll-up property on
the source row is itself subject to `visible:` (not filled when hidden). Values
are per-request and never cached across principals.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see Acceptance Criteria; plus storetest conformance
comparing the native SQL with the generic fallback on the same store.

**Edge Cases:** no related entities (count 0, exists false, others absent);
dangling edges; related entity of another face under a world; integer values
stored as strings or floats (ignored unless integral numbers); empty strings;
latest_by ties (smallest id wins); pick unset on the latest row (empty); sources
with duplicate ids; empty source list; relation walked inverse.

**Negative Tests:** invalid schemas (each load error); write to a roll-up
(create, patch, update, MCP, data-entry); list `filter[...]`/`sort` on a roll-up
→ 400; automation condition reading a roll-up → load error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** SQL divergence between backends (mitigated by conformance tests
against the generic fallback on the same store); a missed surface leaking a
hidden aggregate (mitigated: one visibility entry point, two-principal tests);
ETag/304 serving a stale roll-up (roll-ups injected after the ETag; GET skips
304 for types with roll-ups); read-modify-write callers echoing a roll-up back
(writes refuse it with a clear error; data-entry marks it read-only). Effort:
xl.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs-project metamodel guide (rollup section,
difference from computed), CLAUDE.md package table (`internal/rollup`).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-AH4INS RR-D0ZVQV (critical); RR-EYHB61 RR-XDJGTI
RR-XA8QT8 RR-OT7E2L RR-0YBVN9 RR-WDXGFG RR-8S7OYK RR-1XRIS9 RR-VG8RYD
(significant; RR-VG8RYD wont-fix: the brief requires SQL aggregation); minor/nit
RR-MHHU8J RR-EJB3GQ RR-KXO16V RR-RPNDXT RR-TTHRSL RR-YK036Z RR-B25UZX RR-K1UNVG
RR-P1K0K7 RR-FQX97W RR-GP7CVI.

Plan changes from the review: values are a side map merged into the wire objects
only (never `entity.Entity`); the related query is always the request's world;
Require/PickRequire are judged on the ranked prime row; an empty condition holds
for no row; sum overflow yields no value on every backend; roll-ups are left out
of the predicate entity record type, so every predicate surface refuses them at
load; data-entry config `where`/`sort`/ `group_by` refuse them; list rows
compute roll-ups only with `rollups=1`; stored keys named like a roll-up are
dropped from responses.
