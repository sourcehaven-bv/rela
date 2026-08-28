---
id: PLAN-CDUI97
type: planning-checklist
title: 'Planning: Scheduler run-state gets its own storage service: per-task rows, atomic outcome writes, out of the general KV blob'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- A `schedulerstate` package: a `Store` interface at per-task granularity, a
KV-backed implementation (fs/desktop), a pgstore implementation (postgres), and
a `schedulerstatetest` conformance suite every backend must pass.
- Migration of the existing `scheduler-state.json` document into the new
storage, on first read, preserving in-flight retry ladders.
- Rewiring `internal/scheduler` onto the new store: `IsDue`, `recordSuccess`,
`recordFailure`, `pruneOrphanedState`.
- Updating the `state_kv` migration header, which currently claims scheduler
bookkeeping as a tenant.

OUT:
- Moving `recordSuccess`/`recordFailure` into `runTaskJob` (the executing-node
change) — that is TKT-7XLVP7, which depends on this. This ticket makes the write
*safe*; the follow-up moves *who* calls it.
- Leader election (DEC-OVFGFW).
- `(task_key, run_at)` occurrence keying — rejected; `lastRun` already gives
catch-up.
- Any change to `state.KV` itself. Adding CAS there would widen a seam four
unrelated consumers share.

**Acceptance Criteria:**

1. **Per-task writes.** `RecordSuccess("a", …)` leaves task `b`'s record
byte-identical. Test: seed two tasks, record a success for one, assert the
other's stored record is unchanged.
2. **A stale writer cannot regress a newer last-run.** Test (postgres):
`RecordSuccess(t, T2)` then `RecordSuccess(t, T1)` where `T1 < T2`; assert the
stored value is still `T2`.
3. **Concurrent writers do not lose updates.** Test (postgres): N goroutines on
separate connections each record a success for a *different* task; assert all N
records exist afterwards. This is the test that fails today.
4. **Success clears the ladder; failure does not touch last-run.** Preserves the
existing semantics from `recordSuccess`/`recordFailure` and BUG-ZKK2UL.
5. **Migration preserves in-flight ladders.** Test: seed a legacy
`scheduler-state.json` with a task mid-ladder (failures=3, next_retry set), open
the new store, assert all three values survive.
6. **fs/desktop unchanged.** Existing `internal/scheduler` tests pass without
modification.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the external research was done for DEC-OVFGFW (Oban,
River, Solid Queue, Quartz, Celery Beat, neoq) and its conclusion is recorded
there. This ticket is the in-codebase consequence and needs no separate survey.

**Existing Solutions:**

*The decisive prior art is `internal/userstate` (TKT-CXD0A4).* It solved the
identical problem — per-record state, outside the graph, tiered backends — and
its KV backend documents the exact bug this ticket fixes
(`internal/userstate/kvuserstate/kv.go:20-29`):

> "two processes can read the same document, each apply their own change, and
> the second write clobbers the first ENTIRELY — not just the contended key."

Its postgres migration (`migrations/0009_next_action_state.sql:1-9`) states the
tier split in the same terms: three tables rather than one JSON document
"because this backend exists specifically for the MULTI-PROCESS deployment …
Row-level upserts make concurrent writers safe without coordination."

So the shape is settled by precedent, not invention:

| Piece | Model to copy |
|---|---|
| Package layout | `internal/userstate` — interface + `kv…`/`mem…` subpackages + `…test` suite |
| Interface style | `userstate.Store` — narrow methods, ctx first, injected `now` |
| Conformance suite | `userstatetest.RunAll` (CLAUDE.md requires one per backend) |
| pg backend | `pgstore/userstate.go` + a numbered migration |
| Wiring | `appbuild/userstate_{postgres,nostore}.go`, `storeUserStateFor(st)` |

Other options considered and rejected:

- **Add CAS to `state.KV`.** Rejected: widens a seam shared by the render cache,
settings, logo and CalDAV aliases; every backend plus `ValidatedKV` and the
`statetest` suite would implement a primitive one consumer wants.
- **Per-task keys in `state.KV`.** Rejected as the *primary* mechanism: `state.KV`
is exactly `Get`/`Put`/`Delete` (`internal/state/state.go:27-35`) with **no List
or prefix scan**, so `pruneOrphanedState` could not enumerate stored tasks — it
can only diff against `schedules.yaml`, which is precisely what prune must not
rely on when nodes disagree mid-rollout. Viable for the fs tier *if* an index
key is maintained, but that reintroduces a shared document.
- **An external library.** None applies: this is ~200 lines against two backends
rela already owns.

**Relevant concepts:** `background-jobs`, `store-backends`. Decision:
DEC-OVFGFW.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

New package `internal/schedulerstate`:

```go
// RunState is one task's scheduling record.
type RunState struct {
    LastRun   time.Time // last SUCCESSFUL start; zero = never run
    Failures  int       // consecutive failures; 0 = healthy
    NextRetry time.Time // zero = no retry pending
}

type Store interface {
    Get(ctx context.Context, task string) (RunState, error)
    All(ctx context.Context) (map[string]RunState, error)
    RecordSuccess(ctx context.Context, task string, start time.Time) error
    RecordFailure(ctx context.Context, task string, failures int, retryAt time.Time) error
    Prune(ctx context.Context, keep []string) ([]string, error)
    Close() error
}
```

Notes on the shape, each deliberate:

- **`RecordSuccess` takes the run's START time**, not completion — preserving the
existing rationale (a task starting 23:59 and finishing past midnight must not
land on the next day). Guarded conditionally on postgres: `UPDATE … WHERE
last_run < $new`, so a stale node cannot regress it.
- **`RecordFailure` takes the computed `failures` and `retryAt`** rather than
incrementing internally. The ladder (`retryDelay`, the BUG-ZKK2UL clock-jump
guard) stays in `internal/scheduler` where it is already tested; the store
persists a decision rather than making one. This keeps the store free of
scheduling policy — the same split the queue seam already uses.
- **`All`** exists for `pruneOrphanedState` and for the startup log; it is the
method `state.KV` cannot provide, and the reason for a real backend rather than
per-task KV keys.
- **`Prune(keep)`** returns what it removed so the existing "pruned state for
tasks no longer configured" log survives.
- **No wall clock inside the store** — `now` is always supplied, following
`userstate`'s stated rule and for the same reason (deterministic conformance
tests across backends).

Backends:

- `internal/schedulerstate/kvschedulerstate` — one document through `state.KV`,
same shape as today. Honest about being single-writer, like `kvuserstate`. This
is the fs/desktop tier and its behaviour is unchanged.
- `internal/store/pgstore/schedulerstate.go` + migration `00NN_scheduler_state.sql`
— one row per task, `PRIMARY KEY (task)`. Conditional update for
`RecordSuccess`.
- `internal/schedulerstate/schedulerstatetest` — `RunAll(t, factory)`, run
against both, with the postgres arm DB-gated and wired into `just test-postgres`
**and the CI postgres job** (the gap TKT-YOED3R just closed for `internal/jobs`;
do not repeat it).

Migration of existing data: on first `Get`/`All`, if the new storage is empty
and `scheduler-state.json` exists, import it and leave the legacy key in place
(do not delete — a rollback must still find it). Import is idempotent.

**Files to modify:**

- NEW `internal/schedulerstate/{schedulerstate.go,doc.go}`
- NEW `internal/schedulerstate/kvschedulerstate/kv.go`
- NEW `internal/schedulerstate/schedulerstatetest/schedulerstatetest.go`
- NEW `internal/store/pgstore/schedulerstate.go` + `migrations/00NN_scheduler_state.sql`
- NEW `internal/appbuild/schedulerstate_{postgres,nopostgres}.go`
- `internal/scheduler/{scheduler.go,state.go,config.go}` — rewire onto the store
- `internal/appbuild/appbuild.go` — wiring + accessor
- `internal/store/pgstore/migrations/0008_state_kv.sql` — header no longer claims
scheduler bookkeeping
- `.go-arch-lint.yml`, `.testcoverage.yml`, `justfile`, `.github/workflows/ci.yml`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Task names** — from operator-authored `schedules.yaml`. Already validated by
`ParseConfig` (non-empty, unique). They become a primary-key column on postgres
(parameterised, never interpolated) and a JSON map key on fs. On the fs tier a
name is *not* a path segment under this design (one document), so no traversal
surface is added. If the fs implementation ever moves to per-task files, names
must pass `isSafePathSegment` first — noted so the constraint is not
rediscovered.
- **Legacy state document** — `parseState` already treats a corrupt file as
empty rather than failing; the importer keeps that behaviour.
- Per CLAUDE.md, config contents are not secret: task names may appear in logs
and errors. Run-state carries no entity content and no credentials.

**Security-Sensitive Operations:**

- Postgres writes go through parameterised pgx queries; no dynamic SQL.
- The table lives in the store's schema, so schema-per-tenant scopes it for free
(same property `state_kv` relies on).
- No ACL gate: this is operator-tier scheduling metadata, not graph content —
consistent with `state_kv` and `next_action_*`, neither of which is ACL-checked.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| Criterion | Test |
|---|---|
| 1 Per-task isolation | Conformance: seed two tasks, write one, assert the other unchanged |
| 2 No regression | Conformance (pg): out-of-order `RecordSuccess`, assert newest wins |
| 3 Concurrent writers | pg integration: N goroutines, distinct tasks, separate conns; assert all N present |
| 4 Ladder semantics | Conformance: success clears failures/next-retry; failure leaves last-run |
| 5 Migration | Unit: legacy JSON with a mid-ladder task → all three values survive |
| 6 fs unchanged | Existing `internal/scheduler` suite passes untouched |

Integration (not just units): the existing
`internal/appbuild/scheduler_e2e_test.go` runs a real Lua task through a real
queue; extend it to assert the outcome lands in the new store, so the wiring is
covered end to end rather than only the package.

**Edge Cases:**

- Task never run: `Get` returns a zero `RunState`, no error (distinct from a
failed read — the caller must not treat "never ran" as "error").
- Task renamed in `schedules.yaml`: old record is orphaned, `Prune` reclaims it;
new name starts fresh. Correct, and worth documenting — a rename means a re-run.
- Two nodes, different `schedules.yaml` (mid-rollout): each would prune the
other's tasks. **Resolution: `Prune` is called only at startup and only for
tasks absent from config, and it is the one operation that stays advisory — it
must not delete a record whose `last_run` is newer than this process's start.**
Pins the rule the ticket flagged as open.
- Zero/negative `failures`, zero times: normalise, never persist a negative.
- Clock jump backwards: the existing `retryAt.Sub(now) > maxRetryDelay` guard
stays in the scheduler and is re-verified under concurrent update.
- Empty task name: rejected by the store (defence in depth; `ParseConfig`
already rejects it).

**Negative Tests:**

- `Get`/`RecordSuccess` with an empty task name → error, no write.
- Corrupt legacy document → import yields empty state, no panic, no data loss
of the *new* storage.
- Closed store → every method returns `ErrClosed` (matches `userstate.ErrClosed`).
- Postgres unavailable mid-run → error surfaces to the scheduler, which logs and
continues; a failed state write must not kill the scheduler goroutine.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| Migration loses in-flight ladders on upgrade | Import on first read, keep legacy key, idempotent; pinned by criterion 5 |
| A rollback to an older binary silently reverts to the stale document | Legacy key is retained, so the old binary still works; state diverges but nothing is lost. Document it. |
| The conformance suite runs only in-memory and misses backend divergence | DB-gated pg arm wired into `just test-postgres` **and** the CI postgres job — the exact gap TKT-YOED3R closed for `internal/jobs` |
| Prune deletes a live node's tasks mid-rollout | Advisory prune rule above; never delete a record newer than process start |
| Scope creep into TKT-7XLVP7 | The executing-node change is explicitly OUT; this ticket only makes the write safe |
| A failed state write wedges the scheduler | Errors are logged and the tick continues, matching today's `saveState` behaviour |

**Effort:** m — one new package with two backends, one migration, a conformance
suite and rewiring. No new external dependency.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `CLAUDE.md` — the storage-tier rule ("scheduler state has its own store,
not the general KV") belongs beside the existing background-jobs rules
- [x] `docs/postgres-backend.md` — the multi-writer section already describes
the scheduler limitation; update as the fix lands
- [x] `internal/store/pgstore/migrations/0008_state_kv.sql` — header must stop
claiming scheduler bookkeeping
- [x] ~~docs/cli-reference.md~~ (N/A: no command changes)
- [x] ~~docs/metamodel.md~~ (N/A: not a metamodel change)
- [x] ~~docs/data-entry.md~~ (N/A: no UI change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] ~~All critical/significant findings addressed in plan~~ (findings recorded as RR entities; the plan is being revised against them before implementation)

**Design Review Findings:** RR-Y4417I (critical), RR-JKT6PZ (critical),
RR-9XGRXW (critical), RR-R43942, RR-UUWC92, RR-3HLCZB, RR-DPNJO0.
