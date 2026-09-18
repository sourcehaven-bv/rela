---
id: PLAN-P5RMKX
type: planning-checklist
title: 'Planning: Section sort: plus one declared order per enum, on every sort path'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:

1. `applyV1Sorting` stops comparing byte-wise and delegates to the existing
   type-aware comparison, so the REST list path agrees with search and
   dashboards on enum, integer, date and boolean properties.
2. The pushed-down SQL path orders enums by declared position, so a paged list
   reads the same whether or not the request was pushdown-eligible.
3. The enum's declared values participate in the derived index name, so editing
   `values:` in `schema.yaml` rebuilds the index instead of silently
   de-optimising the query.
4. `Sort []SortSpec` on `ViewSection`, applied before any row cap.

OUT of scope:

- Per-use-site `values:` ordering. Rejected on measurement; see the ticket.
- Changing `store.GraphQuery.OrderBy`'s byte-wise contract or the `storetest`
  conformance suite. The store keeps knowing nothing about enums.
- `filter.Sort` (the CLI single-key path) and `graphquerynaive.Order`. Neither
  serves the list API. Named here so a reviewer knows they were considered.
- Natural-number sort on strings.

**Acceptance Criteria:**

1. **The two sorters agree.** A list of entities with an enum property, sorted
   via `/api/v1/entities?sort=<enum>`, returns the same order as the same
   entities sorted through the search/dashboard path.
   *Test*: seed one fixture, sort it both ways, assert identical id sequences.
   *Mutation*: revert `applyV1Sorting` to byte-wise; the test must fail.

2. **Pushed and Go paths agree, page for page.** Extend
   `TestListPushdown_MatchesGoPathPageForPage` with an enum sort key, asc and
   desc, across page boundaries.
   *Test*: the existing differential harness, with an enum property added to the
   fixture metamodel.
   *Mutation*: emit the byte-wise `ORDER BY` while the Go path ranks; the test
   must fail on page 1.

3. **Declared order, not alphabetical.** An enum declared `todo, doing,
   blocked, done` sorts in that order, not `blocked, doing, done, todo`.
   *Test*: explicit sequence assertion on a fixture whose declared order differs
   from its alphabetical order.

4. **A schema edit does not silently de-optimise.** Reordering or inserting a
   value in `values:` changes the derived index name, so the reconciler drops
   the stale index and creates the correct one.
   *Test*: `listIndexName` returns different names for two metamodels differing
   only in enum value order. Plus an EXPLAIN test proving the generated index is
   used (required by root `CLAUDE.md` for any newly supported SQL shape).
   *Mutation*: omit values from the hash; the name-difference test must fail.

5. **Section `sort:` orders rows before the cap.** A `display: nested` section
   with `sort:` emits the top-sorted rows, not an arbitrary prefix.
   *Test*: a fixture with more children than `nestedChildPreview`, asserting the
   surviving rows are the sorted-first ones.
   *Mutation*: apply the sort after the cap; the test must fail.

6. **Null placement and tiebreak are preserved.** See the two defects below —
   this is the criterion that keeps AC2 honest.
   *Test*: rows with a missing property, asc and desc, plus rows with equal sort
   keys and out-of-order ids.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the research is recorded in the ticket body (Postgres
measurements, rejected alternatives) rather than a separate RES entity, because
it is a decision record for this one design rather than a survey.

**Existing Solutions:**

*In the codebase (the decisive finding).* `internal/filter/sort.go` already
implements declared-order enum sorting: `buildEnumIndex` (`:65`) maps value →
declared position, reading `propDef.Values` for an inline enum or
`meta.Types[...].Values` for a custom type; `compareEnums` (`:88`) compares by
index. `compareByPropDef` (`:333`) also routes date, integer and boolean to
type-aware comparisons. Verified by probe, not by reading.

So this ticket writes **no new comparison code**. It deletes a duplicate.

*The delegation pattern is already written.* `queryservice.sortEntitiesMulti`
(`queryservice.go:306`) builds the `map[string]*metamodel.EntityDef` that
`filter.SortMulti` needs and calls it. `applyV1Sorting`'s call site
(`api_v1.go:518`) has `a.Meta()` in scope, so the same shape works there.

*The stale-index pattern is already written.* `uniqueIndexShape`
(`derivedschema.go:90`) versions the unique-index definition so that changing
the DDL renames the index; the reconciler then drops what it no longer computes.
Added for BUG-HC6I2T. AC4 applies that existing idea to enum values.

*The differential harness is already written.*
`TestListPushdown_MatchesGoPathPageForPage` (`listpushdown_test.go:64`) already
runs every request shape at two page sizes across three pages against both
paths. AC2 adds rows to its table rather than building a harness.

*External research.* Four Postgres idioms were measured on a 200k-row replica of
rela's real index shape; results and sources are in the ticket. Summary:
`array_position` and a rank lookup table both force a sequential scan and are
the two most-recommended answers on the web. Native pg enums are fastest but
cannot be reordered without recreating the type, which operator-edited
`schema.yaml` rules out. An indexed `CASE` rank matches byte-wise performance
and serves both directions from one index via a backward scan.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Four steps, in dependency order. Steps 1 and 2 are the bugfix; 3 is the feature.

*Step 1 — unify the Go sorters.* Replace `applyV1Sorting`'s comparison body with
a `filter.SortMulti` call, building `entityDefs` the way `sortEntitiesMulti`
does. Two behaviour differences must be reconciled first, because
`filter.SortMulti` as written would break paging (see the two defects below).

*Step 2 — push the same order into SQL.* `queryplan` gains an enum-aware order
spec carrying the declared values; `store.OrderSpec` grows an optional rank list;
pgstore emits a `CASE` over it in both `ORDER BY` and `createListIndexDDL`, and
`listIndexName` hashes the values. Backends that cannot rank decline the sort and
fall back to the Go path, exactly as they do for integers today.

*Step 3 — section sort.* `Sort []SortSpec` on `ViewSection`; the section builder
sorts each collection through `filter.SortMulti` before applying any cap;
validation mirrors `columns:`.

**Two defects in `filter.SortMulti` that must be fixed first**

Both found by probe during planning, and both silently corrupt paging if the
delegation lands as-is. Neither is visible on the search path that uses it today,
which is why they have survived.

1. **Null placement differs by direction.** `filter.SortMulti` puts rows missing
   the property LAST in both directions. `applyV1Sorting`, `graphquerynaive` and
   SQL all put them last ascending and FIRST descending (SQL's default). Measured:

   ```
   filter.SortMulti asc : C(todo) E(doing) A(done) B(nil) D(nil)
   filter.SortMulti desc: A(done) E(doing) C(todo) B(nil) D(nil)   ← nils still last
   ```

   Delegating without fixing this makes the Go path disagree with the pushed path
   on every descending sort over a sparse property, which is AC2's failure mode.

2. **No id tiebreak.** Equal sort keys keep input order — measured
   `TKT-10, TKT-2, TKT-1`. `applyV1Sorting` and SQL both break ties by id
   ascending. Without a tiebreak, two rows with the same status have no defined
   order, so a row can appear on both page 1 and page 2, or on neither. This is
   the defect that makes AC6 non-optional.

   Note `filter.SortByID` uses `natsort.Less` (natural order), while
   `applyV1Sorting` and SQL use plain byte order. The tiebreak must be **byte
   order**, not natsort, to match SQL. Do not reuse `SortByID` for it.

Fix both inside `filter`, since the search path benefits from a defined order
too. Guard with direct unit tests before touching `applyV1Sorting`, so the
change is proven in isolation.

**A third defect, found while planning: non-ISO date formats are wrongly
pushdown-eligible.**

`queryplan.StringShaped` (`queryplan.go:119`) admits date and datetime
properties because "byte order IS its order" — true for ISO 8601, false for any
other layout. `metamodel.PropertyDef.Format` (`types.go:762`) lets an operator
declare a Go layout such as `02/01/2006`, and `filter.SortMulti` honours it via
`compareDates` while SQL compares the raw text. Measured:

```
non-ISO format  agree=false
  go       = [31/12/2025  05/01/2026  10/02/2026]   ← chronological
  bytewise = [05/01/2026  10/02/2026  31/12/2025]   ← wrong
```

This is a **pre-existing** bug, not one this ticket introduces: today both paths
are byte-wise on the API list path, so they agree with each other while both
being chronologically wrong. Step 1 makes the Go path correct, which converts a
silent wrongness into a visible divergence between the pushed and Go paths —
exactly the defect `listpushdown.go:15-40` says eligibility must prevent.

So `StringShaped` must decline a date/datetime whose `Format` is not the ISO
default (`metamodel.DefaultDateFormat` / `DefaultDatetimeFormat`). Declining is
the fail-safe direction: it costs pushdown on an unusual config and keeps the
two paths in agreement. Cover it in AC2 with a non-ISO fixture.

ISO dates and mixed date/datetime columns were probed and DO agree, so the
narrowing is limited to explicitly non-default formats.

**Alternatives considered:**

- *Teach `applyV1Sorting` about enums.* Rejected: keeps two comparison rules
  alive, which is the defect this ticket exists to remove.
- *Decline pushdown for enum sorts* (the integer precedent). Rejected: a `CASE`
  rank is indexable at the same cost as byte-wise (measured), so declining would
  give up paging pushdown for no gain. Kept as the fallback if step 2 proves
  harder than expected — it degrades performance, never correctness.
- *Per-site `values:`.* Rejected on measurement; recorded in the ticket.
- *Native pg enum columns.* Rejected; recorded in the ticket.

**Files to modify:**

| File | Change |
| --- | --- |
| `internal/filter/sort.go` | Direction-aware null placement; byte-order id tiebreak |
| `internal/dataentry/api_v1.go` | `applyV1Sorting` delegates to `filter.SortMulti` |
| `internal/store/graphquery.go` | Optional rank list on `OrderSpec`; document the contract |
| `internal/queryplan/queryplan.go` | Emit enum rank in the order spec / index spec; `StringShaped` declines non-ISO date formats |
| `internal/store/pgstore/graphquery.go` | `CASE` rank in `ORDER BY` |
| `internal/store/pgstore/derivedschema.go` | `CASE` in index DDL; hash values in `listIndexName` |
| `internal/store/graphquerynaive/naive.go` | Rank-aware ordering to keep backends in step |
| `internal/dataentryconfig/config.go` | `Sort []SortSpec` on `ViewSection` |
| `internal/dataentryconfig/validate.go` | Validate section sort properties |
| `internal/dataentry/sections.go` | Sort collections before the cap |

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation |
| --- | --- | --- |
| `sort=` query param | HTTP, untrusted | Already parsed to `(property, direction)` pairs; property must resolve in the metamodel or pushdown declines. Never interpolated into SQL. |
| Enum `values:` | `schema.yaml`, operator-authored | **Becomes SQL text in a `CASE` and in index DDL.** See below. |
| `sort:` in `data-entry.yaml` | operator-authored | Validated against the source type at config load. |

**Security-Sensitive Operations:**

*Enum values reaching SQL is the one real concern in this ticket.* The `CASE`
arms are built from `values:`, which is operator-authored config, not end-user
input. Two different rules apply depending on where it lands:

- In `ORDER BY`, values MUST be bound parameters via the existing `b.arg(...)`
  builder, never string-concatenated. This is mechanical and non-negotiable.
- In the derived index DDL, parameters are not available — DDL cannot be
  parameterised. The existing code faces the same problem and solves it with
  `quoteLiteral` / `quoteIdent` / `safeDDLName` (`derivedschema.go:285-290`).
  Reuse those; do not invent new escaping.

Threat model note: an attacker who can edit `schema.yaml` already has the
operator's shell and does not need a SQL injection. So this is defence in depth
and a correctness requirement (a value containing a quote must not break DDL
generation), not a privilege boundary. A value with an embedded quote is a
realistic accident, and `quoteLiteral` is what makes it a non-event.

No ACL surface changes. Sorting reorders rows the principal can already see; the
pushed query remains the principal's compiled read query, and the scoped count
is unchanged. Row-gating happens before ordering on both paths.

No error message gains config or data content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Level |
| --- | --- | --- |
| 1 two sorters agree | Same fixture through `/api/v1/entities?sort=` and the search path; identical id sequences | Integration, real handler |
| 2 pushed == Go | Enum rows added to `TestListPushdown_MatchesGoPathPageForPage` | Integration, both paths |
| 3 declared order | Explicit sequence assertion, declared order ≠ alphabetical | Integration |
| 4 index identity | `listIndexName` differs for two value orders; EXPLAIN shows index use | Unit + postgres-gated |
| 5 sort before cap | Nested section with more children than the preview cap | Integration, real view pipeline |
| 6 nulls + tiebreak | Sparse property asc/desc; equal keys with unordered ids | Unit in `filter`, then integration |

**Edge Cases:**

- Property missing on some rows; JSON null vs absent key (both are "no value").
- Value present in data but absent from `values:` (legacy/removed) — sorts last,
  alphabetically among themselves. Already true; pin it.
- Empty string as an enum value.
- Enum with one value; enum with zero values (`buildEnumIndex` returns nil → byte
  order fallback).
- A value containing a single quote, reaching both `ORDER BY` and index DDL.
- Multi-key sort mixing an enum and a date.
- A date property declaring a non-ISO `format:` (must decline pushdown).
- Sort property declared on one entity type but not another in a mixed result set
  (`comparePropValues` has a `typeRank` path for this).
- Equal sort keys spanning a page boundary — the paging-corruption case.
- A section whose collection is empty, or shorter than the cap.

**Negative Tests:**

- `sort=` naming an unknown property: pushdown declines, Go path leaves order
  untouched. Current behaviour; must not change.
- Section `sort:` naming a property absent from the source type: config load
  error, matching how `columns:` fails. Where `determineTargetType` returns `""`
  (multi-`to:` relation), skip validation rather than guessing.
- Integer property sort stays on the Go path (existing
  `listpushdown_test.go:137` case must keep passing).

**Integration approach:** every AC except 4 and 6 drives the real HTTP handler
through the real ACL gate, matching how TKT-M0WMEE's budget tests are written.
AC4's EXPLAIN half is postgres-gated on `RELA_TEST_DATABASE_URL` and skips
without it, like the rest of the pgstore suite.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Severity | Mitigation |
| --- | --- | --- |
| Paging corruption from the missing tiebreak | **High** | AC6, fixed in `filter` first and unit-tested before delegation |
| Descending-sort divergence from null placement | **High** | AC6; same fix, both directions asserted |
| Stale index after a schema edit (measured 650× slowdown) | **High** | AC4; hash values into the index name, reusing the `uniqueIndexShape` pattern |
| Non-ISO date formats diverge once the Go path is correct | **High** | `StringShaped` declines them; non-ISO fixture in AC2 |
| Enum values reaching DDL unescaped | Medium | Reuse `quoteLiteral`; edge case with an embedded quote |
| Behaviour change visible on upgrade | Medium | Intended. Release note; see the ticket's open question |
| Backends drifting out of step | Medium | `graphquerynaive` updated with pgstore; differential test covers both |
| Scope creep into a per-site override | Low | Explicitly out of scope, with measurements recorded |

**Effort:** m. Step 1 is small and the comparison logic already exists; step 2 is
the bulk (three packages plus an EXPLAIN test); step 3 is mechanical.

One risk worth naming plainly: the EXPLAIN test and the postgres half of AC4
cannot be verified on this machine without a database. Postgres.app is available
locally and was used for the planning measurements, so this is runnable, but CI
is the authority.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — document `sort:` on a view section beside the
      existing `sort:` docs (`:1463`, `:2049`), and state that an enum sorts in
      declared order with unknown values last.
- [x] `docs/metamodel.md` — note that the ORDER of `values:` is now
      load-bearing, not just the set. This is the change most likely to surprise
      an operator: reordering `values:` reorders every list sorted on it.
- [ ] `docs/postgres-backend.md` — only if the derived-index section names index
      shapes; check during implementation.
- [x] Release note — enum-sorted list views change order on upgrade.

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
