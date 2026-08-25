---
id: REV-ADRH9C
type: review-checklist
title: 'Review: Investigate SQLite as a third store backend for single-server and desktop deployments'
status: done
---

<!-- @managed: claude-workflow v1 -->

**Investigation ticket.** Nothing ships; the deliverable is evidence plus
DEC-LFSYNY. Items about shipping code are marked N/A with a reason rather than
silently ticked.

## Automated Checks

- [x] `go build ./...` (default build) — **PASS**. The spike is behind
`//go:build sqlitespike`, so no normal build sees it.
- [x] `go vet ./internal/store/...` — **PASS**
- [x] `go vet -tags sqlitespike ./internal/store/sqlitespike/` — **PASS**
- [x] Conformance harness — `RunTxStressTest` (30s), `RunTxRollbackTests`,
4 fuzz targets (68k execs): all pass. Full matrix in
`internal/store/sqlitespike/RESULTS.md`.
- [x] `just arch-lint` — **5 notices, ALL from the spike**, each
"File /internal/store/sqlitespike/*.go not attached to any component in
archfile". Verified by moving the directory aside and re-running: **"OK - No
warnings found"**. Expected — `.go-arch-lint.yml` has no component for a
throwaway package, and adding one would imply the package is permanent.
Resolution: the branch is never merged, so the archfile stays untouched.
TKT-G91TBK adds a real component for the real backend.
- [x] ~~`just test` / `just coverage-check` full suite~~ (N/A: no production
code changed. The default build is byte-identical apart from a `go.mod`
`require` line, which `go mod tidy` removes on the branch that is never merged.)

## Code Review

- [x] Reviewed — `go-architect` design review ran BEFORE any code was written
(the correct order for a spike, where a wrong harness produces confident wrong
evidence). 4 critical + 4 significant findings, all addressed in PLAN-ZLVJC3
before implementation. See that checklist's Design Review table.
- [x] Findings addressed

**Design review paid for itself twice, and both are worth recording:**

- **C1** — the originally planned separate Go module under `.ignored/` could
not have compiled: Go's internal rule is import-path-prefix based and neither
`replace` nor `go.work` relaxes it. The spike would have failed at `go build`
before producing any evidence.
- **C3** — the originally planned primary arm (`MaxOpenConns(1)`) makes the
spike's own highest-risk unknown **unobservable**: with one connection nothing
can contend, so `busy_timeout` is never exercised and the `BEGIN IMMEDIATE`
control proves nothing. **Confirmed in execution**: that arm HANGS, on
`database/sql` pool starvation. Had it stayed primary, the hang would have been
read as "modernc's locking is broken" — the exact opposite of what the evidence
shows.

**One defect found and fixed during implementation.** `FuzzRelationKeyCollision`
seed #4 failed: `CreateRelation` accepted an empty relation type because the
spike skipped `storeutil.ValidateRelationType`. Fixed by validating relation
type and both endpoint IDs. Carried into TKT-G91TBK as a "do not rediscover"
item — evidence that the conformance harness catches what a hand-written backend
misses.

No `review-response` entities: findings were resolved directly, before
implementation. Had any been deferred or disputed they would be RR-xxxx per the
review protocol.

## Acceptance Verification

Each criterion PASS/FAIL with evidence. Full detail in the ticket body.

| AC | Verdict | Evidence |
|----|---------|----------|
| 1 — required vs optional surface enumerated | **PASS** | 26 mandatory methods + 11 optional capabilities; effort anchors memstore 1,068 / fsstore 2,996 / pgstore 6,169 non-test lines |
| 2 — driver decision | **PASS** | `modernc.org/sqlite`, `CGO_ENABLED=0`, 6/6 targets cross-compile, FTS5/JSON1/R-tree verified |
| 3 — `Transactor` semantics | **PASS** | 30s soak: 31,106 commits / 22,036 pair txs, no deadlock; `RunTxRollbackTests` passes → pgstore's tier. Deferred control arm FAILS, proving `BEGIN IMMEDIATE` load-bearing |
| 4 — change-feed decision | **PASS** | Single-process, lock-enforced. `unique:` is an untransacted scan (`unique.go:82`, zero `.Tx(` sites) → no backstop if multi-process |
| 5 — wiring approach | **PASS** | `sqlite` build tag mirroring the postgres pattern; NOT the desktop default; CI isolation assertions specified in TKT-G91TBK |
| 6 — go/no-go + estimate | **PASS** | **GO** (DEC-LFSYNY). TKT-415WA7 `m` → TKT-G91TBK `l` → stage 4 unestimated |

**Not verified, and recorded as such:** arm F (WAL on a network/sync
filesystem). No network storage available (operator decision, 2026-08-23). It
remains an **assumed** limitation; `TestJournalMode` honours `RELA_SPIKE_DB_DIR`
and is ready if a mount appears. Mitigation is in TKT-G91TBK: refuse or warn at
startup when `journal_mode` is not `wal`, turning a silent corruption risk into
a clear error.

Also unmeasured by design, and footnoted on the AC-3 result so it cannot be
overread: attachments inside a Tx, `RenameEntity`, `Close` under an open Tx.

## Documentation

- [x] ~~User-facing docs~~ (N/A: nothing ships. `docs/sqlite-backend.md`,
the CLAUDE.md backend table row, and README guidance are scoped into
TKT-G91TBK.)
- [x] Decision recorded — DEC-LFSYNY, with measurements rather than adjectives
- [x] Follow-ups filed and linked — TKT-415WA7 (ready), TKT-G91TBK (backlog,
`depends-on` the former)
