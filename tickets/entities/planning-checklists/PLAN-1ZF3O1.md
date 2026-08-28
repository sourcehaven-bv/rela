---
id: PLAN-1ZF3O1
type: planning-checklist
title: 'Planning: Computed properties in schema.yaml: derived, non-editable, stored and indexed, with chained derivation and cycle detection'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: schema-declared entity-local computed scalar properties; a typed,
Lua-compatible expression subset; inferred same-entity dependencies; chained
evaluation; load-time cycle/type/profile validation; recomputation on create,
update, patch, sync/apply and automation/cascade writes; persistence and normal
indexing; rejection at authored-write surfaces; read-only data-entry
affordances; schema-shape hashing; documentation; and SQL-portability metadata
on compiled programs as groundwork for later SQL lowering.

Out of scope: relation rollups, graph/store reads, query-time virtual
properties, an SQL renderer or database-side recomputation, automatic bulk
recomputation after schema changes, calendar/feed behavior changes, and a new
RRULE widget.

**Acceptance Criteria:**

1. A scalar `computed:` expression in an entity property compiles at project load;
a create/update stores its typed result. Test integer multiplication, string
concatenation, date/RRULE result, nil/unset, and validation of result type.
2. Dependencies are inferred from accepted `entity.<field>` IR reads. Test A -> B
-> C evaluation order independent of YAML map order.
3. Direct and indirect cycles, unknown fields, unsupported/list/file result types,
statements, dynamic access and impure/unknown functions fail compilation with
property-qualified errors.
4. Authored values for computed fields are rejected through Manager create/update/
patch/apply and the data-entry, MCP, CLI, Lua/import/sync paths that use it.
Computed values produced internally remain persistable.
5. Create/update automation changes are reflected before final persistence and
transition/unique validation; computed `required` and `unique` properties see
their final values.
6. Computed values travel through ordinary store notifications and are therefore
searchable/filterable/sortable without index-specific code. An integration test
writes and searches a derived value.
7. Data-entry field affordances mark computed properties non-writable and the SPA
never presents an editable control for them.
8. A compiled program exposes required capabilities and SQL portability. Arithmetic/
concatenation/property access are portable; `rrule_next` makes a program valid
for computed use but non-portable. No SQL rendering ships in this ticket.
9. Changing `computed:` changes the metamodel shape hash and produces a migration
delta; bulk recomputation is documented as a follow-up/operator migration.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-KWWT4J

**Existing Solutions:**

The chosen basis is rela's own `internal/predicate` mini interpreter: a strict
gopher-lua expression parser lowered into typed immutable IR with compile-depth
and evaluation-step budgets. `internal/predicatefns` already owns
metamodel-to-type mapping, typed entity binding and pure date functions
including `rrule_next`. `internal/statemachine.Compile` is the
compile-once/inject-required precedent. Odoo's explicit computed-field
dependencies and Terraform's computed attributes were reviewed; rela can avoid
duplicate dependency declarations because its accepted AST forbids dynamic
attribute keys and can expose exact field references.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Extend `internal/predicate` without changing existing boolean consumers: retain
`Compile` as boolean-only and add `CompileValue` with an expected result type.
Add typed arithmetic and string-concatenation IR nodes/evaluation, inferred
field references, and capability metadata. Operator/function semantics are
target-neutral.
2. Add compilation profiles/validation. Computed permits pure entity-local scalar
expressions. SQL portability is metadata/validation over the same IR, never a
second language or altered semantics. Host functions declare portability;
`rrule_next` is computed-valid but SQL-nonportable.
3. Add `internal/computed` mirroring statemachine: compile each entity type's computed
fields against its full record type, verify target type, derive
computed-to-computed edges, topologically sort, reject cycles, and evaluate
against the evolving candidate.
4. Add `Computed string` to `metamodel.PropertyDef`; reject it on relation, list and file
properties. Include it in `PropertyShape` and hash/compare logic.
5. Inject a required computed evaluator into `entitymanager.Deps`. Recompute after
defaults/templates and again after automations, before transition/unique/final
validation and persistence. ApplyEntity recomputes before validation/unique
checks.
6. Reject caller-authored computed keys before recomputation so internal materialized
values are distinguishable from input. Keep one Manager-level invariant and add
surface-specific early errors where those surfaces already validate property
names.
7. Reuse existing `_fields[].writable` rather than changing the API contract; computed
fields resolve false and render display-only/are omitted as inputs.

Alternatives rejected: full Lua VM (larger attack/resource surface and no sound
static dependencies/SQL lowering); explicit `depends_on` (duplicated drift-prone
metadata); probe evaluation (branch-dependent and unsound); lexical scanning
(not syntax-safe); and a separate formula language (duplicates the existing mini
interpreter).

**Files to modify:**

`internal/predicate/{compile,walk,ir,eval,program,value,env}.go` and tests;
`internal/predicatefns/{env,bind,evaluator}.go`; new `internal/computed/`;
`internal/metamodel/{types,loader,shapeprojection,shapecompare}.go` and tests;
`internal/entitymanager/{manager,core,apply}.go` plus wiring/tests;
`internal/appbuild/appbuild.go`; CLI/MCP/data-entry validation and affordance
files; frontend form rendering tests where necessary; `docs/metamodel.md`,
data-entry docs, and architectural guidance.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Expressions are operator-authored schema input and pass through a default-deny
AST walker. Only declared entity fields, whitelisted pure functions and
supported scalar operators compile. Dynamic indexing, statements, loops,
functions, tables outside named arguments, graph reads and all I/O remain
rejected. Runtime stored values are coerced through declared metamodel types;
off-type inputs become nil and result validation still runs before persistence.

**Security-Sensitive Operations:**

Evaluation performs no I/O and has a fixed node-step budget. Dependency cycles
fail at load. Computed values are trusted system writes but can derive from
fields hidden by read ACL; documentation warns that the computed field's own
visibility must not be broader unless that disclosure is intended. SQL
portability cannot change semantics; future lowering must preserve nil,
overflow, date/timezone, short-circuit and collation rules or refuse the
expression.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

Predicate unit tests cover typed operators, overflow/divide-by-zero,
concatenation, result profiles, dependency collection, capability classification
and budgets. Computed compiler tests cover ordering/cycles/types/portability.
Entitymanager tests cover create, patch, update, apply, automation
re-evaluation, required and unique. Surface tests cover HTTP/MCP/CLI/Lua
rejection and read-only affordances. A store/search integration test proves a
materialized derived value is indexed through normal events. Shape-projection
tests prove expression changes move the hash.

**Edge Cases:**

Missing/nil dependency; nil result removes the stored key; zero/negative/maximum
integers; checked overflow; divide/modulo by zero; unicode and null bytes in
concat; custom enum strings; date versus datetime formatting; exhausted RRULE;
dependency on a later YAML property; self and multi-node cycles; unchanged
recomputed result; concurrent evaluations of immutable programs; and schema
visibility leakage.

**Negative Tests:**

Unknown/dynamic fields, unknown/impure functions, statement syntax, unsupported
output type, list/file computed fields, result-type mismatch, cycle, arithmetic
type mismatch, overflow, direct authored value at every write surface, and
nonportable expression under an SQL-required profile all fail with stable
contextual errors.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Semantic drift between Go evaluation and future SQL: one typed IR, explicit node/
function portability, golden semantic tests, and refusal over approximation.
- Write-path coverage gaps: required evaluator in Manager plus table-driven coverage of
create/update/patch/apply/cascade and production wiring.
- Automation ordering/double writes: recompute at candidate construction and after
automation; assert audit/index behavior and idempotence.
- Integer/date ambiguity: checked typed operators and bound clock values, never DB
wall-clock or implicit timezone.
- Scope growth: SQL renderer and bulk migration remain explicit follow-ups.

Effort: L.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/metamodel.md` - declaration, semantics, examples and staleness
- [x] Data-entry documentation - computed fields are read-only
- [x] `CLAUDE.md` / package docs - typed IR profiles and SQL-portability rule
- [x] Migration docs - expression changes move shape; recomputation is explicit
- [x] CLI/API references reviewed; update only if user-visible error/shape changes

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** No open findings. Review specifically added: exact
semantic parity as a prerequisite to SQL portability; computed-field visibility
disclosure; checked integer errors; after-automation recomputation; and Manager
coverage for sync/ cascade paths. These are incorporated above.
