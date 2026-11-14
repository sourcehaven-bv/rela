---
id: PLAN-54H0XJU
type: planning-checklist
title: 'Planning: Record types: pluggable append-only record store for time series and ledgers'
started: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: the `records:` schema section, the `records.Store` contract, file,
sqlite and postgres backends, the conformance suite, the service with
owner-based ACL, bucketed aggregates, the ledger constraint (balance,
reverse, period lock), owner rename and delete, retention, and REST.
Out (TKT-EV8U8T4): MCP, Lua, CLI, rollup source, SPA panel, named backend
registry, `records:read`/`records:append` permissions, entity ref fields.

**Acceptance Criteria:**
1. Every backend passes `recordstest.RunAll` (atomic batch, order, cursor,
   group claim incl. concurrent, rekey, drop, expire).
2. A hidden owner gives no rows and no aggregate contribution
   (`TestRecords_AggregateExcludesHiddenOwner`, `TestRecords_GatedThroughPolicy`).
3. An unbalanced batch is refused whole; a reversal nets to zero; a row in a
   closed period is refused (`TestLedger_*`).
4. `just arch-lint` passes.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-BB8A86O

**Existing Solutions:**
The side-store pattern of leases, piles and channels: a domain package with
the Store contract, a shared SQL backend over `internal/sqlexec`, a file
backend and a conformance suite. TimescaleDB and similar are possible later
backends behind the same contract (see RES-BB8A86O).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
As in the ticket's Design section. Group claims use a `record_groups` table
with `ON CONFLICT DO NOTHING` in the append transaction.

**Files to modify:**
`internal/metamodel/records.go`, `internal/records/...`, pg migration 0047,
`internal/sqlitedb` v37, `internal/appbuild/records*.go`,
`internal/dataentry/records_handler.go`, `.go-arch-lint.yml`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
HTTP bodies (1 MiB cap, unknown keys refused, numbers kept exact), query
parameters (RFC 3339 times, whole-number limit), schema field types. Invalid
input is a 400 naming the field.

**Security-Sensitive Operations:**
Reads gate on owner readability; writes need update on every readable face
of the owner; aggregates gate owners before folding; file paths escape every
byte outside `[A-Za-z0-9_-]`; SQL is parameterised.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1: `recordstest.RunAll` per backend. 2: service tests with a fake gate and an
appbuild test over a real `acl.yaml`. 3: `TestLedger_*`. 4: `just arch-lint`.

**Edge Cases:**
Concurrent group claims, ids with path characters, empty group, legs with
different ts, rows of one batch with equal ts, unbounded aggregates (capped).

**Negative Tests:**
Bad schema (table test), unbalanced batch, reused group, closed period,
hidden owner (404), read-only owner (403), bad cursor.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
Group keys are global per type, so a 409 reveals a used key (documented).
Generic aggregates load rows in Go (capped; native SQL in TKT-EV8U8T4).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: the guide is written in this change; a docs-checklist is created at review)

**Documentation Impact:**
New guide GUIDE-records (docs/records.md, linked from README.md).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was reviewed with the user in RES-BB8A86O; code and security review ran on the implementation)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run; review findings are tracked as review responses)

**Design Review Findings:** N/A
