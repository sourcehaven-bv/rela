---
id: DOCS-AHXC4B
type: docs-checklist
title: 'Docs: SQLite store backend (TKT-G91TBK)'
status: done
---

<!-- @managed: claude-workflow v1 -->

**Nothing user-selectable ships in this ticket.** The package exists but no
binary links it — that is TKT-L1A3PH. So user-facing docs are deferred rather
than skipped, and the items below are marked N/A with the ticket that owns them.

## Code Documentation

- [x] Package doc written — `internal/store/sqlitestore` opens with what the
backend IS (single-process, between fsstore and pgstore), the single-writer
guarantee and *why* it exists, and the three measured spike findings.
- [x] **The failure modes are documented where the mistake would be repeated**,
not in a commit message. `tx.go` carries the connection-poisoning explanation
with its measured numbers (10/10 connections poisoned, leaked row visible) next
to the deferred rollback that prevents it — because the natural "simplification"
is to delete that defer.
- [x] Non-obvious decisions explained at the point of use: DSN-vs-`db.Exec`
PRAGMAs in `Open`; `BEGIN IMMEDIATE` in `Tx`; the sidecar-not-database-file
reasoning in `lock.go`; why `json_each` rather than a constructed JSON path; why
`HighestID` folds case deliberately; why observers defer past commit.
- [x] The Windows `Overlapped` constraint is recorded — safe only because the
call is synchronous and immediate-fail — since adding a retry loop is exactly
the change someone makes without noticing.

## Project Documentation

- [x] ~~`docs/sqlite-backend.md`~~ (N/A → TKT-L1A3PH: nothing to operate yet)
- [x] ~~CLAUDE.md storage-backend table row~~ (N/A → TKT-L1A3PH: the table is
keyed by build tag, and there is no sqlite tag until that ticket)
- [x] ~~README backend selection guidance~~ (N/A → TKT-L1A3PH)
- [x] Architecture recorded where the project looks — `.go-arch-lint.yml` gains
the component with a **deliberately narrower** `mayDependOn` than pgstore's, and
a comment saying so: widening it to reach versioning or state has to be argued
for rather than drifted into.

## Knowledge Capture

- [x] Findings that would otherwise be rediscovered are in the follow-up ticket,
not stranded on a branch. TKT-L1A3PH carries the `cli/db_nonpostgres.go` seam (a
sqlite build would inherit *"requires the PostgreSQL build"*) and a table of the
three `!postgres` no-ops that need **deciding** rather than inheriting.
- [x] The review's structural lesson is captured in the SHARED suite, not this
package. `RunTxAbnormalExitTests` and `RunTxObserverIsolationTests` exist
because `RunTxRollbackTests` only ever returns an *error* from `fn` — it never
panics, cancels, or inspects observers, so a backend could claim the strong tier
and still be permanently unusable after a panic. Every backend is now held to
it.
- [x] `RESULTS.md` on the spike branch remains the measured record behind the
decisions; the package doc cites it rather than restating it.
