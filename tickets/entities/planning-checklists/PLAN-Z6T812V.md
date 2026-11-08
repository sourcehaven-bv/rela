---
id: PLAN-Z6T812V
type: planning-checklist
title: 'Planning: Flows M1b: run store on file, pg and sqlite'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

PR 2 of the four M1 PRs (see TKT-XHANLJU, section Split). A flow run's execution
state (active state, generation, entry time, timer schedule, reminder records)
lives in a run store keyed by run id, not in the graph. The run id is the id of
the run's `_flow-run` entity, which TKT-BC5G8RP adds; this store does not mint
ids.

In scope:

- `internal/flowrun`: the `Store` contract, the `Run` value, key and value
checks, errors. The stored execution state is a `flow.Instance`, so the engine
functions (`flow.Read`, `flow.StepSchedule`, ...) apply unchanged.
- `internal/flowrun/flowruntest`: the conformance harness every backend runs.
- Backends: `kvflowrun` over `state.KV` with a cross-process lock (fs,
memory and desktop tiers), `sqlflowrun` (shared SQL), `pgflowrun` and
`sqliteflowrun` (driver adapters). New tables `flow_runs` and `flow_definitions`
by a pgstore migration and a sqlitedb rung.
- Definition snapshots: the first run of a flow version stores the
version's definition in the same step, so a run can finish on the version it
started on after `flows.yaml` changes.
- Retention: `Delete` drops a run's state on completion or cancel;
`PruneDefinitions` drops snapshots no stored run uses.
- `flow.ValidName`, shared by `flowcfg` and `flowrun`.
- CI: the postgres job runs the flowrun suites and fails on a skip, as it
does for the comment backends.

Out of scope (later tickets): wiring the store into `appbuild.Services` and
every consumer (TKT-9IT3LN8, the runtime, which is its first user); the
`_flow-run` type, ACL and provenance (TKT-BC5G8RP); timers loop, triggers,
inbox, `rela flow start | runs | cancel` (TKT-9IT3LN8). Moving channel sessions
onto this store is not planned: a session lives inside its channel row and moves
in the same transaction as the message that causes it, which a separate store
cannot offer.

**Acceptance Criteria:**

1. Every backend passes `flowruntest.RunAll`: create, get, compare-and-set
update, delete, due listing, filtered listing with a cursor, subject rekey,
definition snapshots and pruning, key and value refusals. Tests:
`TestConformance*` in kvflowrun, pgflowrun, sqlflowrun (over sqliteflowrun).
2. Concurrent writers: of N concurrent updates at one rev exactly one
wins; of N concurrent singleton creates for one flow and subject exactly one
wins; two store instances over one backing store (two processes) see each
other's writes. Tests: `flowruntest` concurrency cases with `Harness.Second`.
3. A run's state survives a reopen of the backing store (fs disk KV,
sqlite file). Tests: `TestReopen` per backend.
4. A damaged KV document is an error naming the key, never a silent reset.
Test: `TestDamagedDocument` in kvflowrun.
5. Migrations: pgstore migration 43 and sqlitedb v35 create the tables on
a fresh and on an existing database. Tests: the existing ladder tests plus
`TestMigrate_v34ToV35FlowRuns` in sqlitedb.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: FEAT-J0O10QI settles
that execution state lives in a run store on file, pg and sqlite.)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:**

- Channel session storage (`internal/channels` with `sqlchannels`,
`pgchannels`, `sqlitechannels`, `filechannels`): one shared SQL implementation
with thin driver adapters, a `?`-placeholder querier, a dialect lock statement,
microsecond BIGINT times, `ListSessionsDue` ordered by due time and key. The run
store follows this layout.
- `internal/lease/kvlease`: one JSON document in a `state.KV`, read-modify-
write under a one-slot semaphore and a cross-process `Locker` (`filelock`).
Runtime state on fs uses `state.KV`, per `.claude/rules/storage.md`. `kvflowrun`
follows kvlease.
- Workflow engines (Temporal, Camunda Zeebe, AWS Step Functions) keep an
execution row per run with an optimistic version and a timer index; completed
executions move to history. Same split here: the run store holds live state, the
`_flow-run` header holds history.
- No library: the store is a small table plus a KV document.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`flowrun.Run{ID, Flow, Version, Subject entity.Ref, Singleton, CreatedAt,
UpdatedAt, Rev, Instance flow.Instance}`. Rev starts at 1 and rises on every
update.

`flowrun.Store` (declared beside its types, like `channels.Store`, because
`flowruntest.RunAll` is the one definition of the contract every backend
satisfies; consumers declare their own narrow interfaces):

- `Create(ctx, NewRun) (Run, error)`: NewRun carries the definition bytes;
the first run of a (flow, version) stores them, a later one must carry bytes
with the same SHA-256 digest (`ErrDefinitionConflict`). An existing id is
`ErrExists`. A singleton run (`per_subject: one`) refuses a second stored
singleton run of the same flow and subject id, on any face, with
`ErrSubjectBusy`. A subject id holding `MaxRunsPerSubject` (100) runs of the
flow is `ErrSubjectFull`.
- `Get(ctx, id)`.
- `Update(ctx, id, expectRev, flow.Instance, at)`: one compare-and-set
statement on rev; lost race `ErrConflict`, missing `ErrNotFound`.
- `Delete(ctx, id, expectRev)`: compare-and-set; the retention point on
completion and cancel.
- `ListDue(ctx, until, after DueCursor, limit)`: runs with `NextDueAt`
set and at or before until, ordered by due time then id, after a keyset cursor
so a run the caller cannot advance does not starve the rest.
- `List(ctx, Query{Flow, SubjectID, After, Limit})`: ordered by id,
cursor after an id, limit 1 to `MaxPage`.
- `RekeySubject(ctx, oldID, newID)`: moves every run of a renamed subject;
a singleton clash is `ErrSubjectBusy`, a cap overflow `ErrSubjectFull`, and
nothing moves.
- `Definition(ctx, flow, version)` and `PruneDefinitions(ctx, before)`.

Compare-and-set rather than a decider callback under the lock (the channel
store's pattern): the runtime reads the subject through the store and evaluates
guards before it writes, and slow reads do not belong inside a transaction
(`store.Store.Tx` rule). A lost race re-reads and decides again.

SQL: one implementation in `sqlflowrun` over the `?`-placeholder querier of the
new `internal/sqlexec` package, whose `pgexec` and `sqliteexec` adapters the
channel backends now share too (one copy of the transaction and rebind code). On
pg, Create takes a per-schema definitions advisory lock shared and a per-subject-id
lock (per subject id, not per flow, so a rekey that locks both ids first
excludes creates of every flow on them); PruneDefinitions takes the definitions lock exclusive, so a
prune never drops the snapshot a concurrent create relies on, and creates on
different subjects do not wait on each other. sqlite's BEGIN IMMEDIATE already
serializes writers. Tables: `flow_definitions (flow, version, def, digest,
saved_at)` and `flow_runs (id, flow, version, subject_id, subject_face,
singleton, next_due_at, created_at, updated_at, rev, doc)` with a foreign
key to the definition, a partial unique index on (flow, subject_id) for
singletons as a backstop, a partial due index, subject, flow and definition
indexes. `doc` is a JSON envelope `{"instance": ...}`, so a later field (a
pending-outputs mark) needs no migration; the due time is a column for the
index. Times are microsecond BIGINT, as in sqlchannels; ids sort bytewise
(`COLLATE "C"` on pg).

KV: two documents, `flow-runs.json` (every run) and `flow-definitions.json`
(snapshots), the definitions written first so the runs document never names a
missing snapshot. Read-modify-write under a one-slot semaphore and a `Locker`
(`filelock`). A cap of 2000 runs keeps the document bounded (`ErrFull`); fs is
the good-enough tier.

Alternatives rejected: storing execution state on the `_flow-run` entity
(settled against: timer writes would version and audit the graph); extending
`channels.Store` (a run has no channel); one KV key per run (no listing in
`state.KV`, so due listing needs an index document anyway).

**Files to modify:** `internal/flow/name.go` (new), `internal/flowcfg/load.go`,
`internal/flowcfg/template.go` (RunID alias), `internal/flowrun/**` (new),
`internal/sqlexec/**` (new),
`internal/channels/{sqlchannels,pgchannels,sqlitechannels}`,
`internal/store/pgstore/migrations/0043_flow_runs.sql`,
`internal/sqlitedb/{sqlitedb.go,migrate.go}`, `.go-arch-lint.yml`,
`.testcoverage.yml`, `.github/workflows/ci.yml`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The store's callers are rela's own runtime (later PR), never a request directly.
Still every write is checked, so a caller bug cannot store what another backend
refuses:

- Run and subject ids: `entity.ValidateID`; face: `entity.ParseFace`.
- Flow, state and reminder or timer names: `flow.ValidName` (allowlist
`^[a-z][a-z0-9_-]{0,63}$`); version: 16 lowercase hex digits.
- Instance: generation and cause not negative; at most `flow.MaxTimers`
restart anchors and `flow.MaxReminders` reminder records.
- Definition bytes: 1 byte to 1 MiB (the flows.yaml cap).
- List and due limits: 1 to `MaxPage`; an invalid query is `ErrInvalid`.

SQL takes every value as a bound parameter; no identifier is built from input.
KV keys are built from validated names only and pass `state.ValidateKey` (the
FSKV root confines them).

**Security-Sensitive Operations:** storage of run state outside the graph.

### Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1)

Assets: the execution state of a run (which state a subject's process is in,
when it is due), which discloses facts about the subject entity, so it is as
secret as the subject's existence. Definition snapshots are config, not secret.

Actors: rela's flow runtime (the only caller; system principal); a person or
agent reaching the store through a later API (not in this PR); a second
rela-server process on the same schema; a tenant sharing the database.

| Threat (STRIDE) | Vector | Mitigation in this PR |
| --- | --- | --- |
| Information disclosure | A later read path lists runs and leaks the existence of a subject the reader cannot see | No read path ships here. The store authorizes nothing, like `channels.Store`; TKT-BC5G8RP gates runs through the `_flow-run` entity's row ACL (`related(_carries, ...)`), and the runtime PR may expose runs only through that gate. Recorded as a carried requirement on TKT-9IT3LN8 |
| Information disclosure | Tenants sharing a database read each other's runs | Tables live in the tenant schema (schema-pinned pool); the pg advisory lock key includes `current_schema()` |
| Tampering | Two processes update one run and one write is lost | Compare-and-set on rev; conformance test with two store instances |
| Tampering | A prune drops the snapshot a concurrent create relies on, so a run cannot load its definition | Per-schema definitions lock, shared in Create and exclusive in Prune, on pg; single writer on sqlite and KV, foreign key from run to definition |
| Tampering | A changed definition replaces the snapshot of a running version | Snapshots are write-once per (flow, version); different bytes are `ErrDefinitionConflict` |
| Denial of service | Unbounded listings or an unbounded KV document | `MaxPage` on every list; 2000-run cap on the KV tier; 100 runs per flow and subject id; definition cap 1 MiB |
| Denial of service | A damaged KV document wedges every flow | Error names the key; the operator restores or deletes it. A silent reset would lose running processes, which is worse |
| Elevation of privilege | Injection through ids or names into SQL or KV paths | Bound parameters; allowlist names; `entity.ValidateID`; `state.ValidateKey` |
| Spoofing / repudiation | n/a | No writes to the graph or audit here; provenance lands with TKT-BC5G8RP |

Residual: erasure on disk follows the sqlchannels caveat (dead tuples, WAL,
freed pages); a deleted run's state can survive until VACUUM. Run state holds
state names and times only, no property values.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria above. pg runs against a private
migrated schema per test (as pgchannels); sqlite against a temp `rela.db` opened
through `sqlitedb.Open`; KV against MemFS and against an on-disk KV with a real
`filelock`.

**Edge Cases:** an unset instance (zero state, generation 0); a run with no due
time is never listed due; equal due times order by id; `until` equal to a due
time includes it; a cursor past the last id returns an empty, non-nil slice; a
faced subject and a faceless one with the same id share one singleton key (a
singleton blocks across faces); rekey with no runs is a no-op; prune keeps a
snapshot a run uses and one saved after `before`; times keep microsecond
precision in UTC; reminder and restart maps round-trip; a nil map reads back
nil.

**Negative Tests:** invalid ids, names, versions, faces, negative generation,
too many reminder records, empty or oversized definition, limit 0 or above
`MaxPage`, update or delete at a stale rev, update or delete of a missing run,
duplicate id, second singleton, definition mismatch, KV cap reached, damaged KV
document.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- The runtime PR may need an operation this contract lacks. Mitigation:
the contract covers the needs listed in TKT-9IT3LN8 (start with `per_subject`,
step, timers, subject delete via `List` by subject, rename, retention; the inbox
reads `_flow-run` headers through the visibility gate, not the store); adding a
method later means adding a harness case.
- A migration on a shared production database. Mitigation: pure CREATE
TABLE IF NOT EXISTS; no backfill; no change to existing tables.
- The pg conformance suite never ran in CI for pgchannels. Mitigation:
add a postgres-job step for `./internal/flowrun/...` that fails on skip.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A for user docs: nothing runs flows yet, and the Flows guide
already says so. The runtime PR documents runs. Package docs carry the contract.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 15 review-responses, all addressed in the plan
above. Significant: RR-HAHY15G (prune vs create: shared/exclusive definitions
lock plus a per-subject lock), RR-R5I7UE0 (listing by state was a second ungated
read path: States dropped, the inbox reads headers), RR-U30RNUZ (room for a
side-effect mark: JSON envelope doc). Minor: RR-7E3BWVM (`Definition` loads a
snapshot alone), RR-X2YUJXP (digest compare, exact version check), RR-67RJROD
(due cursor), RR-RRCTTG2 (per-subject cap), RR-AFFWVWQ (singleton semantics
across faces), RR-DTSDBII (subject id across faces; start vs rename carried to
the runtime), RR-SZ1NFMN (KV split in two documents, prune). Nits: RR-HQMI46V
(single-statement CAS), RR-SVDODML (run ids never renamed), RR-QJG4IOH (typed
`RunID`, caller clock), RR-9YJ1C0Z (`flowruntest`, arch-lint rules), RR-5HNPZGH
(consumers declare narrow interfaces).

Carried to TKT-9IT3LN8: a pending-outputs mark in the state envelope; a snapshot
that no longer compiles strands its run (report, do not crash); start racing a
subject delete or rename; the inbox reads headers through the visibility gate;
runs are exposed only through the header row gate. Carried to TKT-BC5G8RP:
refuse renaming a `_flow-run` entity.
