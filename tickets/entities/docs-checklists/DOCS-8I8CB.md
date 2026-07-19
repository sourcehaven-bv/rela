---
id: DOCS-8I8CB
type: docs-checklist
title: 'Documentation: Tx write-transaction contract on store.Store'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious — the full behavioral contract lives
in the `store.Transactor` godoc (per-backend guarantees, view discipline, the
silent escaped-view hazard, the no-external-I/O rule); fsstore/memstore `tx.go`
headers explain the txMu/lock-order/re-entrancy structure and the reduced
single-user guarantees; pgstore `tx.go` explains the advisory-lock choice,
savepoint mechanics, and the txPending post-commit event buffer;
`writeAdvisoryLockKey` documents why it is distinct from the migrate and sweep
keys.
- [x] Function/type docs if public API — `Transactor`, `Tx` on all three
backends, and every re-exported write wrapper carry doc comments.

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level surface change)
- [x] CLAUDE.md updated — the "No repository or transaction abstractions"
rule now names `store.Store.Tx` as the one sanctioned transaction seam
(DEC-8UIL0) and forbids wrapping it or doing external I/O inside a callback.
- [x] ~~Help text accurate~~ (N/A: no CLI command changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: project has no CHANGELOG file)
- [x] API docs updated — `docs/postgres-backend.md` gained a "Write
transactions" subsection under Multiple writers (deployment-wide advisory lock,
rollback/event semantics, the keep-callbacks-short rule, and the fs
degradation).
