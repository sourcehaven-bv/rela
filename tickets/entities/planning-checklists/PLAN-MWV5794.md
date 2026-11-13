---
id: PLAN-MWV5794
type: planning-checklist
title: 'Planning: Flows M1e: flow worker'
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a flow worker in the long-running hosts (rela-server, each
resident tenant of the multi-tenant server, rela-desktop) that calls the
engine's due rounds under a leader lock; a per-tick cap with a cursor; a poll
that tests a waiting `when:` join again; RR-1D9R0UF and RR-ZRMWUYT; restoring
the text #2198 took out. Out: start triggers and change events (TKT-GYXOTQC),
cancel on subject delete (TKT-8137DHO), retention (TKT-H43D60E), the inbox
(TKT-IVL2LF9), `db load` of runs (TKT-TWLN8UB), the M2 and M4b leftovers
(TKT-YNQPHPP), job steps (TKT-PY2G11W).

**Acceptance Criteria:**
1. A timer escalates on a tick without a person acting (fs, sqlite,
postgres).
2. A run whose output failed is settled on a tick once its backoff passed
and the cause is gone, and the output is written once.
3. A join with `when:` that was false at the arrival continues on a later
tick once the subject changed.
4. Only one process holds the flow worker lock per project or tenant.
5. One tick advances at most 4 rounds of 64 runs, each round after the last.
6. A one-shot process (CLI, stdio MCP) builds no worker.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the worker copies the realtime worker's leader pattern; no open design choice beyond the tenant question below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `internal/appbuild/realtime_worker.go` (leader lock:
`pgstore.TryKeyedLock` keyed by schema, a filelock in the cache dir, a free lock
in memory; takeover loop; `check` of a held lock). `flowengine.Engine.Due`
(timers, pending marks, backoff) and `flowrun.Store.ListDue` with a cursor. The
scheduler's single-leader loop. Libraries such as river or neoq need a postgres
queue and do not fit fs or desktop.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- `flowengine.Engine.DueRound(ctx, after DueCursor) (DueReport, error)`
lists up to `MaxDuePerRound` (64) due runs after the cursor, advances each and
reports `Advanced`, `Failed`, `More` and `Next`. `Due` is one round from the
front.
- Join due time: `put`, the single run writer, sets a forked run's due
time to the earliest of the live branches' due times and the join's (`joinDue`).
A join that holds by its shape (all arrived, an arrival at a final state, `any`
with an arrival) is due 30s after the write, so a crash or failed write between
the arrival and the join firing is picked up. A partly arrived `when:` join is
due after `JoinRecheck` (5m). Neither while a live branch has a pending mark, so
its backoff holds.
- A create cut off by its context keeps its claim (it may have landed).
- The tick cursor carries over to the next tick when the last round was
full, so failing runs at the front cannot starve the rest.
- The flow worker stops first in `Services.Close`. Each held postgres
keyed lock is counted per pool; the n-th needs pool_max_conns >= n + 2.
- `appbuild.flowWorker` holds a call-site interface `dueRounds` and a
`leaderLock`. `newFlowWorker` rejects nil. `buildFlowWorker` returns no worker
unless the process is long-running (`WithBackgroundAutomationJobs`) and has
flows. The lock is `workerLeaderLock(fs, cacheDir, store, "flow-worker",
"flow-worker.lock")`, generalised from the realtime worker.
- `RunFlowWorker(ctx, svc)` runs it next to `RunRealtimeWorker` in serve.go,
tenantmode.go (per resident tenant) and rela-desktop; `Services.Close` stops it.
- A tick every 15s runs up to 4 rounds of 64, each with a 1 minute timeout,
passing the cursor on.
- RR-1D9R0UF: `settleStep` returns early when the token has no pending
mark. RR-ZRMWUYT: `CheckState` rejects a run-level pending mark or claim while
forked, a pending mark or claim on an arrived branch, and duplicate generations.

**Alternatives rejected:** running Due from the scheduler process only (the
desktop has none, and fs projects rarely run it); a jobs.Queue job per due run
(the queue is ephemeral on fs and the engine already persists due times); an
in-engine goroutine (identity and lifetime belong to the host).

**Files to modify:** `internal/flowengine/{engine,branches,step}.go`,
`internal/flowrun/flowrun.go` and `flowruntest`,
`internal/appbuild/{flow_worker,realtime_worker,channels_*,appbuild,flowruntime}.go`,
`cmd/rela-server/{serve,tenantmode}.go`, `cmd/rela-desktop/main.go`, the
#2198 text (GUIDE-flows, GUIDE-mcp-server, cli, mcp, flowview, frontend, i18n).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No new input. The worker reads stored run state
(checked by `CheckState` on every write, now also for forked shapes) and
flows.yaml (already loaded and checked).

**Security-Sensitive Operations:** Writes by the worker: run headers and
outputs, through entitymanager like every flow write.

**Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1).**

- Identity. The worker stamps `system:flow` on its context. The engine
binds identity per write: headers as `system:flow`, outputs as the flow's
`run_as` (buildFlowEngine binds each through the identity binder). The worker
holds no allow-all reader; nothing infers allow-all from the `system:*`
principal. A join `when:` is evaluated over the raw subject, as system routing,
as it already was at an arrival: the load compiled it under the visible: check,
and its verdict is never shown to a person.
- Leader election. One process per project or tenant leads: on postgres a
session advisory lock keyed by schema and "flow-worker", so tenants never
exclude each other; on fs and sqlite a lock file in the project cache dir; in
memory a free lock (one process). The leader checks the postgres lock every
takeover interval, between ticks, and stops on loss. Two leaders can overlap for
up to a takeover interval plus one tick; every run write is a compare-and-set on
the run's rev, and a create output is claimed before it is written, so an
overlap cannot write a create twice.
- Unbounded work. One tick handles at most 4 x 64 runs, each round bounded
by a 1 minute timeout; the cursor makes a tick visit each run once. A run that
keeps failing backs off up to 1 hour, so it cannot starve others.
- Crash mid-step. A run is marked pending before its step writes; a crash
leaves the mark and the due time, and a later tick settles it (a set output is
written again, a claimed create is not).
- Multi-tenant. A tenant's worker runs while the tenant is resident, like
its realtime timers. An evicted tenant's due runs wait until it is opened again;
TKT-ZOU5BCT adds a sweep that opens a tenant with due runs.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** `internal/appbuild/flow_worker_test.go` runs on the default
(fs), sqlite and postgres builds: TimerEscalates (1), RecoversPendingOutput (2),
JoinWhenBecomesTrue (3), OneLeader (4), OnlyLongRunningHosts (6).
`flow_worker_internal_test.go`: TickCap (5, with a fake engine),
OnlyTheLeaderRuns, nil rejection. `flowengine/due_test.go`: DueRound cap and
cursor, the join poll and no poll without `when:`. `flowrun_test.go`: the forked
shapes `CheckState` rejects.

**Edge Cases:** a round that hits the cap exactly; a failing round stops the
tick; a join that is still false at the poll is polled again; a join without
`when:` gets no poll; a lost postgres lock stops leading.

**Negative Tests:** a second worker does not take a held lock; a task title
conflict keeps the output pending until it is gone; `newFlowWorker(nil, ...)`
fails; illegal forked run states fail `CheckState`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Load from polling joins: only partly arrived `when:` joins are
polled, every 5 minutes. A slow store makes a tick long: rounds have a timeout
and the next tick starts after the last ends. A new `CheckState` rule rejects a
stored run written by an older build: the rejected shapes are ones the engine
never wrote. Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-flows (status note, timers, join poll, the flow
worker, retries), GUIDE-mcp-server (`settling`), regenerated into `docs/`; the
CLI and MCP messages and the web app strings.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 10 review-responses linked to TKT-FWQZGAM (2
significant, 7 minor, 1 nit). RR-1AN36IV (stranded join) is addressed by
`joinDue`. RR-RGJ2GZJ (tenants that are not resident) is deferred to
TKT-ZOU5BCT. The minor and nit findings are addressed in the approach above.
