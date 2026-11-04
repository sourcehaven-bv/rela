---
id: PLAN-CEA3KEH
type: planning-checklist
title: 'Planning: Add a decimal property type'
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `type: decimal` in the metamodel (load, `scale`, `min`/`max`,
`list`, `required`, `default`, validation), filter and sort on every backend,
predicates and computed properties, `latest_by`/`pick` roll-ups, kanban
summaries, sync merge, data migration `convert`, configure allowlist, schema
API/MCP/CLI/OpenAPI, the SPA number widget, currency display, and docs. Out
(deferred): roll-up `sum`/`min`/`max` and `where:` over decimal (needs exact SQL
on sqlite), `unique: true` on decimal, fractional bounds, int/decimal mixing in
expressions, dashboard matrix axes, realtime channel message types.

**Acceptance Criteria:**
1. A schema with `type: decimal`, `scale: 2`, `min`/`max` loads; a bad scale or
fractional bound fails to load (metamodel TestParse_DecimalOptions).
2. "12.50" round-trips byte-stable on fs, memory, sqlite and postgres stores
(storetest RunDecimalTextTests).
3. `price>9.5` matches "10"; `price=12.5` matches "12.50"; sort orders 9 before
10 (filter TestMatchDecimal/TestSortDecimal, predicatefns tests).
4. Computed `entity.price * entity.qty` is exact (computed TestEvaluate_DecimalIsExact).
5. The SPA emits decimal text, steps by scale, formats currency exactly, and
sums column summaries exactly (vitest decimal/format/summary/widgets tests).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach follows the integer type's existing special-casing; no competing designs needed a research doc)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: stdlib math/big and BigInt cover the arithmetic)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Go: `math/big.Rat` (stdlib) for exact parse/compare/arithmetic; no third-party
decimal library, to avoid a dependency.
- Frontend: BigInt scaled integers; `Intl.NumberFormat` formats decimal strings
exactly (ES2023).
- Codebase: every `PropertyTypeInteger` special case was mirrored
(metamodel validation/loader, filter match/sort, predicate types, conditionlint,
summaries, syncmerge, datamigration, widgets).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Stored as a string in plain decimal notation, kept
exactly as written (no canonicalisation, so files stay byte-stable). Unquoted
YAML/JSON numbers are read through their shortest float text. Comparisons use
big.Rat. No SQL changes: `metamodel.StringShaped` returns false for decimal, so
every backend filters and sorts decimals on the Go path, like integers.
Alternatives rejected: storing a YAML number (loses trailing zeros and
exactness); canonicalising on write (rewrites hand-edited files); pushing into
SQL (sqlite has no exact decimal type; string order is wrong).

**Files to modify:** internal/metamodel, filter, predicate, predicatefns,
affordances, computed, conditionlint, dataentryconfig, syncmerge, datamigration,
configedit, apiwire, dataentry, mcp, cli, openapi, testutil, store/storetest;
frontend utils/widgets/forms/types; docs-project guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Property values from API/MCP/CLI/files and
literals in config. Allowlist grammar `[+-]?digits(.digits)?`, at most 64
characters; exponents refused, so a value like `1e1000000` cannot make big.Rat
allocate huge numbers. `scale` capped at 30.

**Security-Sensitive Operations:** None new. Read-side ACL is unchanged: decimal
values flow through the same visibility redaction as any property.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria; storetest conformance is the
cross-backend integration test.

**Edge Cases:** Trailing zeros, leading zeros, `+` sign, negative zero,
64-character values, float inputs (0.1), empty values, lists, mixed
integer/decimal sort.

**Negative Tests:** `1e3`, `.5`, `5.`, `1,5`, too-long values, scale overflow,
out-of-range, division in expressions, int/decimal mixing, scale on non-decimal.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- A missed integer special case leaves decimal unsupported somewhere: grep for
every `PropertyTypeInteger` / `'integer'` use; deferred ones are listed.
- Float input loses digits: documented; quoting keeps the exact text.
Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md - New metamodel features (GUIDE-metamodel)
- [x] docs/cli-reference.md - filter comparisons
- [x] docs/data-entry.md - widget table, summaries, currency
- [x] docs/data-migration.md - convert to decimal

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: this work ran as a delegated sub-agent without a review step; the design decisions above are reported to the caller for review before merge)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run, see above)

**Design Review Findings:** none recorded
