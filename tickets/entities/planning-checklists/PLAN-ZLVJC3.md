---
id: PLAN-ZLVJC3
type: planning-checklist
title: 'Planning: Investigate SQLite as a third store backend for single-server and desktop deployments'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

This ticket remains **investigation-only**. Its two unanswered acceptance
criteria (3: precise `Transactor` semantics; 4: the change-feed decision) cannot
be answered honestly from documentation alone — the research named modernc's
locking behaviour as the highest-risk unknown, backed by two open upstream
issues. So the remaining work is a **throwaway spike** whose sole output is
evidence, plus the decision entity that closes the ticket.

IN scope:

- A throwaway SQLite store at `internal/store/sqlitespike/` behind a
`//go:build sqlitespike` tag, on a branch that is **never merged**, implementing
the subset of `store.Store` the Tx contract tests exercise.
- Running `storetest.RunTxStressTest` across six configuration arms, the six
fuzz targets, and `RunTxRollbackTests`.
- A ~60-line sidecar-file advisory lock (the only surviving path to a positive
AC-4 — see the Approach section).
- Three separate performance measurements: cold open, steady-state single
write, bulk load.
- A `decision` entity recording the outcome, linked via `informs`.
- An implementation ticket (or a no-go note) with a grounded estimate.

NOT in scope:

- **Merging anything to `develop`.** This is the isolation invariant — not
"nothing under `internal/`", which was the earlier phrasing and which forced a
module layout that does not compile.
- The stage-2 interface-widening refactor of the three `*pgstore.Store`
assertions. That is real production work and belongs to the implementation
ticket.
- FTS5 search integration, SQL pushdown, versioning implementation.
- `PRAGMA optimize` / query-planner tuning — explicitly not measured here.
- Migration tooling between backends.

**Acceptance Criteria:**

Restating the ticket's open criteria with concrete test scenarios. Criteria 1,
2, 5 and 6 are already satisfied by RES-03TUXO; criterion 2 (driver) was met on
2026-08-22.

1. **(AC-3) `Transactor` semantics stated precisely, alongside fs and pg.**
Test: arm A (multi-connection, `BEGIN IMMEDIATE`) passes
`storetest.RunTxStressTest` at `RELA_STRESS_SECONDS=30` with zero watchdog
deadlock dumps, zero lost updates, and pair-atomicity holding. Output is a
three-row table (fs / sqlite / pg) covering serialization scope, rollback and
event timing — **footnoted with what the stress test does not cover**
(attachments, `RenameEntity`, `Close` under an open Tx, ctx cancellation), so
the decision entity cannot overclaim.

2. **(AC-4) Change-feed decision explicit.** The earlier formulation — "show
a second process violating the `unique:` invariant" — was **unrunnable**, and
verifying that is itself the finding. `unique:` is enforced in
`entitymanager/unique.go:82` by a full `ListEntities` scan, and there are **zero
`.Tx(` call sites in `internal/entitymanager`** (verified), so the scan and the
subsequent write are not transactional. A bare `store.Store` spike has no
`unique:` logic to violate. Replaced by two constructible pieces:

   - **(a) A code-level argument, already complete and stronger than a demo.**
Because scan-then-write is untransacted, the race is not SQLite-specific and not
even multi-process-specific — two goroutines in one process can already
interleave. What multi-process removes is the in-process `txMu` that currently
keeps the window narrow. Therefore a multi-process SQLite backend inheriting
`reconcileDerivedSchemaIfSupported` as a no-op would have `unique:` enforcement
**with no backstop whatsoever**. Recorded in the decision entity with citations;
needs no experiment.
   - **(b) A runnable store-layer experiment:** two processes both
`CreateEntity` with the same ID; assert `store.ErrConflict` is returned exactly
once. This tests SQLite's own `PRIMARY KEY` constraint across processes, which
should pass — an honest and useful finding, since it shows SQLite's
*constraints* are cross-process while the *application scan* is not.

3. **(AC-5) Performance — three measurements, not one.** The earlier single
comparison (10k bulk insert vs. fsstore's index build) compared two different
operations and gated on the wrong axis.
   - **Cold open**: SQLite open vs. fsstore's startup index build. SQLite
should win decisively — avoiding full-graph residency is the stated motivation.
   - **Steady-state single write**: one `BEGIN IMMEDIATE`/insert/commit. This
is the interactive path and the number that actually matters.
   - **Bulk load**: 10k entities in one transaction vs. fsstore writing 10k
markdown files (the comparable operation).

No pass/fail gate on bulk load — TKT-TWIO11 §3 already accepted modernc is
~1.5-1.9x slower than mattn there. The gate is on cold open and steady-state
write.

4. **(AC-6) The `synchronous=NORMAL` durability trade recorded as a decision**,
not left as a tuning knob: in WAL mode it means a crash can lose recently
committed transactions, which matters for a backend whose purpose is replacing
git as the durability and history story.

5. **Go/no-go recorded as a decision entity** with the measured numbers, not
adjectives.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-03TUXO (done) — recommends Option C staged, with this
spike as stage 1 and an explicit go/no-go gate.

**Existing Solutions:**

- **Driver**: `modernc.org/sqlite` v1.57.0 chosen over `mattn/go-sqlite3`;
settled in TKT-TWIO11 §3 with empirical verification. Pure Go, FTS5/JSON1/
R-tree compiled in unconditionally, all six release targets cross-compile at
`CGO_ENABLED=0`.
- **Conformance harness**: `internal/store/storetest` — `RunAll` +
`RunTxStressTest` + six fuzz targets. `Capabilities` has exactly one field
(`Attachments`), so almost nothing is opt-out.
- **Template to copy**: `internal/store/memstore/conformance_test.go` (~70
lines) is the minimal wiring template.
- **Weak-Tx reference impl**: `internal/store/fsstore/tx.go:31-42` — `txMu`
mutex, `lockTx()` helper, every exported write a `defer s.lockTx()()` one-liner.
The spike's Tx should start from this shape and add `BEGIN IMMEDIATE`.
- **Strong-Tx reference**: `internal/store/pgstore/tx.go` (95 lines) plus
`RunTxRollbackTests` (`storetest/tx.go:133`), which is deliberately NOT in
`RunAll`.
- **Graph query for free**: `internal/store/graphquerynaive` — fsstore's
entire adoption is 28 lines (`fsstore/graphquery.go`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

**Location — corrected twice; this is the final form.** The spike lives at
`internal/store/sqlitespike/` behind a `//go:build sqlitespike` tag, on a
throwaway branch that is **never merged**. The isolation invariant is "nothing
merges to develop", NOT "nothing under internal/" — the earlier phrasing forced
a module structure that does not compile.

Two rejected alternatives, both verified empirically rather than assumed:

1. *Separate Go module under `.ignored/` + `replace`* — **impossible**. Go's
internal rule is import-path-prefix based and neither `replace` nor `go.work`
relaxes it: `use of internal package .../internal/storetest not allowed`. The
spike must implement `internal/store.Store` and import `internal/entity`,
`internal/store`, `internal/storeutil` and `internal/store/storetest`, so every
one of those hits the same wall.
2. *Package inside the main module under gitignored `.ignored/`* — compiles
and runs (verified: `go test ./.ignored/spikeprobe/` → `ok`, running the real
`RunTxStressTest` against memstore). Rejected anyway because `.ignored/` is
gitignored, so the evidence artifact evaporates and the branch cannot be shared
or reviewed.

The build tag keeps the spike out of every normal build; the branch keeps it out
of `develop`. `go.mod` gains `modernc.org/sqlite` **on that branch only** —
which is itself useful evidence, since the implementation ticket needs the
`go.sum` weight and the `modernc.org/libc` pinning hazard (upstream #177).

**The required subset is most of the write path, not a thin stub.** Derived by
reading `storetest/stress.go`'s five workers rather than guessing:

| Method | Why the stress test needs it |
|--------|------------------------------|
| `CreateEntity` | Must return `store.ErrConflict` on duplicate ID (`stress.go:168`) |
| `DeleteEntity` | Must return `ErrNotFound` / `ErrHasRelations` correctly (`stress.go:177-178`) |
| `UpdateEntity` | Tx read-modify-write counter path |
| `GetEntity` | Same |
| `ListEntities` | Real `iter.Seq2` with type filtering, called continuously (`stress.go:194`) |
| `Subscribe` | Drained for the whole run at buf 1024 (`stress.go:215`) |
| `Tx` | The thing under test |
| `Close` | Lifecycle |

Relation CRUD is needed only insofar as `ErrHasRelations` must be reachable.
Effort revised **m → l** accordingly; the earlier "few hundred lines" was
optimistic by ~2-3x.

**Event-emission is a designed input, not a discovered detail.** pgstore defers
in-process notifications past commit via a `txPending` buffer
(`pgstore/tx.go:44-56`); fsstore emits inline. Which one the spike picks changes
what the stress test proves, so the spike implements **both** and reports the
difference. This is where the "events must never be emitted under a store lock"
edge case actually bites.

**Nested `Tx` must not touch the pool.** `stress.go` calls `tx.Tx(...)` from
inside an open transaction. Copy pgstore's shape exactly
(`pgstore/tx.go:61-63`): `if s.txPending != nil { return fn(s) }` — return the
*same* view. Acquiring a second connection while the outer transaction holds the
only one is an instant self-deadlock that the watchdog would misreport as a
modernc locking failure.

**Connection/PRAGMA setup**, now the actual variable under test:

- `PRAGMA journal_mode=WAL` — **check the return value**; it silently stays
`delete` if WAL cannot be enabled (see the network-FS arm).
- `PRAGMA busy_timeout=5000`
- `PRAGMA synchronous=NORMAL` — note this means a crash can lose recently
committed transactions. For a backend whose purpose is replacing git as the
durability story, that is a **decision to record**, not a tuning knob.
- `BEGIN IMMEDIATE` to take the write lock up front.
- `PRAGMA optimize` is **deliberately not measured here** (Tx-only spike),
flagged so the implementation ticket inherits it explicitly rather than as a
silent gap.

**Test arms — multi-connection is PRIMARY.** The earlier draft made
`SetMaxOpenConns(1)` the primary arm, which was a confound that would have
invalidated the whole spike: with one connection there is no second connection
to contend with, so `busy_timeout` is never exercised, `SQLITE_BUSY` cannot
occur, and the `BEGIN IMMEDIATE`-vs-deferred comparison proves *nothing*.
Upstream #232 and #192 — the entire reason for spiking — are specifically about
multi-connection contention, i.e. exactly the configuration `SetMaxOpenConns(1)`
makes unobservable.

| Arm | Pool | Tx mode | What it proves |
|-----|------|---------|----------------|
| **A (primary)** | `MaxOpenConns(N)`, N≥4 | `BEGIN IMMEDIATE` + `busy_timeout` | Does modernc's locking hold? The actual question. |
| **B (control)** | `MaxOpenConns(N)` | deferred | Is `BEGIN IMMEDIATE` load-bearing, or cargo-cult? Now genuinely testable. |
| **C (fallback)** | `MaxOpenConns(1)` | either | Does the degenerate config work? Safety net if A fails. |
| **D (ship shape)** | 1 writer + N reader conns, WAL | `BEGIN IMMEDIATE` | The configuration a real backend would actually use. |
| **E (durability)** | as D | `RunTxRollbackTests` | SQLite likely gives real rollback free — would place it at pg's tier, not fs's. Material to the decision. |
| **F (network FS)** | as D, DB on an SMB/iCloud mount | `BEGIN IMMEDIATE` | Does WAL work at all? See risks. |

Alternatives considered and rejected:

- **Answer from documentation alone.** Rejected: #232 and #192 are both open
with no conclusive resolution to cite.
- **Build the real backend in `internal/` on develop.** Rejected: commits to
the direction before the gate.

**Files to modify:**

All on a throwaway branch, none merged:

- `internal/store/sqlitespike/*.go` — the spike, all behind `//go:build sqlitespike`
- `internal/store/sqlitespike/lock_unix.go` / `lock_windows.go` — sidecar flock (see AC-4)
- `internal/store/sqlitespike/RESULTS.md` — raw numbers
- `go.mod` / `go.sum` — `modernc.org/sqlite` + pinned `modernc.org/libc`

`internal/storetest` is imported read-only; nothing existing changes.

**Single-writer advisory lock — pulled INTO scope.** The research deferred this
to implementation. That is unsafe: given AC-4(a) above, the `unique:` half of
the original disjunction is unrunnable, so the lock is the *only* surviving path
to a positive AC-4. ~60 lines, and it surfaces the network-FS question below.

Design, with the traps named:

- **Never lock the DB file itself.** SQLite already holds POSIX advisory locks
on that inode, and on Linux/macOS closing *any* fd to a file drops all of that
process's POSIX locks on it — layering our own lock there risks silently
clobbering SQLite's own.
- Lock a **sidecar** (`rela.db.lock`): `syscall.Flock(fd, LOCK_EX|LOCK_NB)` on
unix, `LockFileEx` with `LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY` on
windows (`golang.org/x/sys/windows`). Split `lock_unix.go` / `lock_windows.go`.
- **"Naming the holder" is a separate problem.** OS advisory locks report
*that* a lock is held, never *by whom*. Write PID + hostname + start time into
the lock file **before** taking it, and treat that content as advisory on read —
a stale PID may have been recycled. Do not promise a clean error message without
this.

**Network filesystems are a go/no-go input, not a footnote.** The research
pivoted to single-process *desktop*, and desktop project directories routinely
live in iCloud Drive, Dropbox, OneDrive or an SMB share. WAL mode requires
shared memory (the `-shm` file) and is documented as not working over most
network filesystems — `PRAGMA journal_mode=WAL` can silently stay `delete`.
`flock` is unreliable on the same filesystems, so S1's lock degrades there too.
fsstore has no such problem: markdown files sync fine. This is a concrete
product regression versus fsstore on the exact target deployment, so arm F tests
it and the decision entity must either report it works or state "unsupported on
network/sync filesystems" as an accepted limitation.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The spike takes no untrusted input — inputs are test fixtures generated by
`storetest`. No network, no user data, no config parsing.

For the eventual real backend (recorded here so the implementation ticket
inherits it, not actioned now):

- **Entity IDs / relation types** → must go through
`storeutil.ValidateID` / `ValidateRelationType`. The fuzz suite treats
`ValidateID` as the validity oracle: anything it rejects, the store MUST reject.
Parameterized SQL throughout; never string-concatenate an ID.
- **Attachment file names** → `store.ValidateFileName` +
`NormalizeFileName`, and `CapAttachmentReader` at `MaxAttachmentBytes` (64 MiB)
as the mandatory backstop every backend enforces.
- **The DB file path** → derived from `cfg.Paths`, never from user input.
This is the one genuinely new surface versus pgstore (whose DSN is env-only,
deliberately, so credentials never reach `ps`). A SQLite path is not a
credential, but it is a filesystem write target and must be confined to the
project dir.

**Security-Sensitive Operations:**

- **File creation** — the spike writes a DB file to a temp dir only.
- **No credential handling** — SQLite has no DSN/password, which removes the
`RELA_DATABASE_URL` class of concern entirely rather than relocating it.
- **ACL is unaffected**: read-side gating lives in `internal/visibility`
decorators at the wiring site, not in the store. A new backend inherits it by
construction and must not add per-backend redaction (DEC-ZBI39P).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Arm / scenario | Pass condition |
|----|----------------|----------------|
| AC-3 | **A** multi-conn + `BEGIN IMMEDIATE`, `RELA_STRESS_SECONDS=30` | No watchdog deadlock dump; no lost updates; pair atomicity holds |
| AC-3 | **B** multi-conn + deferred | Expected to show `SQLITE_BUSY` or deadlock. If it passes, `BEGIN IMMEDIATE` is cargo-cult and we say so |
| AC-3 | **C** `MaxOpenConns(1)` | Fallback only. A pass here is near-zero evidence — documented as such |
| AC-3 | **D** 1 writer + N readers, WAL | The ship shape. Read throughput must not collapse |
| AC-3 | **E** `RunTxRollbackTests` | If it passes, SQLite is at pg's tier not fs's — material to the decision |
| AC-3 | Six fuzz targets, 30s each | No crashes, no invariant violations |
| AC-4 | Code-level argument (a) | Citations recorded; no experiment needed |
| AC-4 | Two processes, same ID (b) | `ErrConflict` returned exactly once |
| AC-4 | Advisory lock | Second process refused with an error naming PID + host |
| AC-5 | Cold open vs. fsstore index build | SQLite decisively faster |
| AC-5 | Steady-state single write | Acceptable interactive latency |
| AC-5 | Bulk load vs. fsstore 10k file writes | Recorded, not gated |
| AC-6 | **F** DB on SMB/iCloud mount | `journal_mode=WAL` return value checked; result recorded either way |

**Edge Cases:**

- **Concurrent access** — the central case; the stress test's five workers
(Tx RMW counters, multi-write transactions with injected failures, nested Tx,
plain writes, reads, a draining subscriber) cover it.
- **Nested `Tx`** — must return the *same* view without acquiring a
connection (`pgstore/tx.go:61-63`). With `MaxOpenConns(1)` a pool acquire here
is an instant self-deadlock that the watchdog would **misreport as a modernc
locking failure**. Predicted, not discovered.
- **`journal_mode=WAL` silently not applied** — check the returned mode, never
assume the PRAGMA took.
- **Escaped view after `Tx` returns** — documented as the one misuse that
fails silently. Not defended against; the spike records whether SQLite makes it
loud (closed-tx error, like pg) or silent (like fs/mem).
- **Context cancellation mid-`Tx`** — fsstore ignores the ctx entirely
(`fsstore/tx.go:31` takes `_ context.Context`); SQLite's driver *will* honor it
and abort. A real behavioural divergence the contract does not currently
describe — record it.
- **`Close` under an open Tx.**
- **Subscriber buffer full** — events must drop, not block, and never be
emitted under a store lock.
- **Fuzz-arm resource exhaustion** — `FuzzFactory` is `func() store.Store`
with no `*testing.T`, so no `t.TempDir()`; each iteration builds a **fresh**
store (`fuzz.go:56`). At fuzzing rates that is thousands of SQLite files. Needs
explicit `Close` + temp-dir lifecycle, or `:memory:` for the fuzz arm only —
which changes locking behaviour, so it must NOT be the stress arm.
- **Unicode / null bytes / control chars in IDs** — the fuzz targets treat
`storeutil.ValidateID` as a directional oracle (`fuzz.go:33`).
- **Empty store** — `LastModified` returns zero time.

**Negative Tests:**

- `errStressRollback` is injected after both writes of a pair. Under the weak
contract the writes may persist; pair atomicity must still hold.
- Second process while the lock is held → clear, actionable error naming the
holder; never silent corruption.
- Arm B (deferred) → expected to fail. The failure is the evidence.
- Attachments inside a Tx are **not** tested here, and that is called out in
the AC-3 footnote: a 64 MiB `AttachFile` (`store.MaxAttachmentBytes`) inside
`BEGIN IMMEDIATE` would hold the global write lock for the whole transfer —
exactly the "do not perform slow external I/O inside fn" hazard
(`store.go:210-211`). A design constraint for the implementation ticket.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| modernc's locking doesn't hold under sustained multi-connection load (upstream #232/#192) | Medium | Exactly what arm A measures. A failure here is a **successful spike** producing a no-go — far cheaper than discovering it after a 3k-line backend. |
| **Spike configured so it cannot observe its own risk** | — | **Caught in design review.** The earlier plan made `MaxOpenConns(1)` primary, which makes `busy_timeout` unreachable and `SQLITE_BUSY` impossible — the exact configuration under which #232/#192 cannot reproduce. Multi-connection is now the primary arm. |
| WAL unavailable on network/sync filesystems (iCloud, Dropbox, SMB) | **Medium-High** | Arm F. This is a real product regression versus fsstore on the desktop target, and `flock` degrades on the same filesystems. May itself be a no-go input; at minimum an explicitly accepted limitation. |
| Spike bug misreported as a driver failure | Medium | The nested-Tx self-deadlock (M2) is predicted in the edge-case list. On any watchdog dump, first check the spike's own nested-Tx path against `pgstore/tx.go:61-63` before blaming modernc. |
| Passing stress test read as "the Tx contract holds" | Medium | A pass is weak positive evidence; a failure is decisive. AC-3's output table carries a footnote naming what is untested: attachments, `RenameEntity`, `Close` under Tx, ctx cancellation. |
| Fuzz arm exhausts file descriptors | Medium | `FuzzFactory` builds a fresh store per iteration with no `t.TempDir()`. Explicit `Close` + temp-dir lifecycle, or `:memory:` for the fuzz arm only. |
| Steady-state write latency unacceptable | Medium | Measured directly in AC-5. Mitigations before declaring failure: prepared statements, `synchronous=NORMAL` (with its durability trade recorded as AC-6). |
| Spike scope creeps into the real backend | Medium | Hard rule: **nothing merges to `develop`**. Build tag + throwaway branch. |
| Spike proves the model works, read as approval to ship | Low | The decision entity must state that stage 2 (widening the three `*pgstore.Store` assertions) is a prerequisite and unestimated until scoped. |
| Effort underestimated | — | **Already materialised**: revised m → l in design review once the required subset was derived from `stress.go` rather than guessed. |

**Effort:** l — revised up from `m`. The "subset" is most of the write path plus
a working `iter.Seq2` iterator plus event fan-out plus the advisory lock, across
six configuration arms. The original "few hundred lines" estimate was optimistic
by roughly 2-3x.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A for this ticket — investigation only, nothing user-facing ships.

Deferred to the implementation ticket, if go:

- `docs/` — a `sqlite-backend.md` guide paralleling `postgres-backend.md`,
including the single-writer constraint and the git-versus-version-history
trade-off (the existing guide's framing at line 194 is the model).
- `CLAUDE.md` — the "Storage backends & build tags" table gains a row, and
the backend-isolation CI assertion gains its sqlite cases.
- `README.md` — backend selection guidance.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Reviewed by `go-architect` before any spike code was written. Four critical and
four significant findings; all addressed in this plan. Every claim below was
independently verified against the codebase rather than taken on trust.

| # | Sev | Finding | Resolution |
|---|-----|---------|------------|
| C1 | critical | Separate Go module under `.ignored/` **cannot compile** — Go's internal rule is import-path-prefix based; neither `replace` nor `go.work` relaxes it. The spike would have failed at `go build` before producing any evidence. | Moved to `internal/store/sqlitespike/` behind `//go:build sqlitespike` on a never-merged branch. Isolation invariant restated as "nothing merges to develop". Verified both the failure and the fix. |
| C2 | critical | The "subset" is most of the write path — `stress.go` needs real `ListEntities` iteration, `ErrConflict`, `ErrHasRelations`, and event fan-out. | Subset enumerated method-by-method from `stress.go`'s five workers. Event-emission (buffer-and-replay vs. inline) promoted to a designed input. Effort **m → l**. |
| C3 | critical | `MaxOpenConns(1)` is a confound that makes the spike's own highest-risk unknown **unobservable** — no second connection means `busy_timeout` is never exercised and the `BEGIN IMMEDIATE` control arm proves nothing. #232/#192 are specifically multi-connection bugs. | Arms inverted: multi-connection is now primary, `MaxOpenConns(1)` demoted to fallback. Added the 1-writer/N-reader "ship shape" arm and a `RunTxRollbackTests` arm. |
| C4 | critical | AC-4's unique-violation demo is **unrunnable**: `unique:` lives in `entitymanager/unique.go:82`, not the store, and there are zero `.Tx(` sites in `internal/entitymanager` (verified) — so a bare `store.Store` has nothing to violate. | Replaced with (a) a code-level argument that is *stronger* than the demo — untransacted scan-then-write means the race isn't even multi-process-specific — and (b) a runnable cross-process `ErrConflict` test. |
| S1 | significant | The advisory lock was deferred to implementation, but after C4 it is the **only** surviving path to a positive AC-4. | Pulled into scope (~60 lines). Design recorded with the traps: sidecar file never the DB file (SQLite holds POSIX locks on that inode; any fd close drops them); PID+host written before locking; holder identity treated as advisory. |
| S2 | significant | A passing stress test is weak positive evidence; attachments, `RenameEntity`, `Close`-under-Tx and ctx cancellation are untested. `Tx`'s ctx is ignored by fsstore but honoured by SQLite — a real divergence. | AC-3's output table now carries an explicit "what this does not cover" footnote. The 64 MiB-attachment-inside-`BEGIN IMMEDIATE` hazard recorded as an implementation constraint. |
| S3 | significant | No arm tested WAL on a network/sync filesystem, though desktop project dirs routinely live in iCloud/Dropbox/SMB — where WAL may silently not engage and `flock` is unreliable. A product regression versus fsstore on the exact target. | Added arm F. May itself be a no-go input; at minimum an explicitly accepted limitation. |
| S4 | significant | AC-5 compared two different operations (bulk insert vs. index build) and gated on the wrong axis. | Split into cold open / steady-state write / bulk load, each against its comparable fsstore operation. Gate moved off bulk load. `synchronous=NORMAL` durability trade promoted to its own decision (AC-6). |

Minor findings folded in without separate tracking: fuzz-arm fd exhaustion
(`FuzzFactory` has no `t.TempDir()`), the nested-Tx self-deadlock predicted
rather than discovered, the `internal/`-vs-`develop` scope phrasing, and `PRAGMA
optimize` marked explicitly not-measured.

No `review-response` entities were created: findings were resolved directly in
the plan before implementation, which is what a design review is for. Had any
been deferred or disputed, they would have been filed as RR-xxxx per the review
protocol.
