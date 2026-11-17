---
id: RES-HVW3B7O
type: research
title: 'Audit log as a service: writer/reader split, SQL in the write transaction, hash chain'
summary: Keep audit.Audit as the write seam; add a hash-chained SQL log (pg + sqlite) with verify/prune/import, move the append into the write Tx fail-closed in a second PR, and add a gated read API in a third.
status: done
---

## Problem

GRC auditors ask two questions rela cannot answer today: who changed what, and
can the log be trusted. The audit log (`internal/audit`) is JSONL under
`.rela/audit/` on local disk, even on postgres and sqlite. It is written
best-effort after the write commits, so a crash or a full disk leaves a change
with no record. Nothing detects an edited or deleted line, and there is no read
API.

## Context

- `audit.Audit` is one method, `Record(Record)`, with no error. About 40 call sites in `internal/entitymanager` call it after the store write returns, outside any transaction. Most entity writes run through `writeChecked`, which opens a `store.Store.Tx` only when a check needs one.
- The database backends already let a side table join the store transaction: flow runs use `pgstore.HandleFor(view)` and `sqlitestore.TxConn(view)` (`appbuild/flowruns_tx_*.go`), and one shared SQL implementation over `internal/sqlexec` serves both databases (`sqlflowrun`, `pgflowrun`, `sqliteflowrun`). Tables are created by pgstore migrations and the sqlitedb DDL, so they sit in the tenant schema.
- `store.ContextWithTxView` carries the transaction view on ctx, so a collaborator can write on the transaction's own handle.
- Tenant mode builds one filesystem sink per tenant schema (`internal/tenant/registry.go`).
- History reads have the read gate and redaction this work needs for the per-entity view (`.claude/rules/versioning.md`). Collection reads need store paging and a budget test (`.claude/rules/collection-reads.md`).
- External sends belong on `jobs.Queue` (`.claude/rules/jobs.md`).

## Options

### A. Keep JSONL, add a hash chain to the files

Each line carries the previous line's hash; a verify command walks the files.

- Pro: small change; works on every backend.
- Con: still best-effort and per machine. Several rela-server processes on one database write separate files, so there is no single chain. No paged query for a read API. Does not close the crash window.
- Effort: S.

### B. SQL audit table written after commit, in its own transaction

A `sqlaudit` log in the tenant schema; `Record` appends synchronously in a
separate transaction under an audit lock.

- Pro: one chain per tenant across processes; queryable; small change to the write path.
- Con: the crash window stays. A failed append cannot fail a write that has already committed.
- Effort: M.

### C. SQL audit table written inside the write's own transaction (recommended)

As B, plus the entity manager writes each entity and relation record inside the
`store.Store.Tx` of the write, through the transaction view. The sink joins the
transaction the same way flow runs do. On postgres and sqlite a failed append
rolls the write back.

- Pro: no change exists without its record. The chain is serialized by the store's write lock plus an audit advisory lock, so it stays linear across processes. Reads are plain indexed queries.
- Con: every audit emission site in the entity manager moves into its transaction and returns an error; writes that ran without a transaction now open one on the database backends. Reverses the best-effort rule, so an audit outage stops writes (recorded as a decision).
- Effort: L.

### D. Store decorator that audits every store write

Wrap `store.Store`; each write method opens a Tx and appends a record derived
from its arguments.

- Pro: covers every store write, including paths that bypass the manager.
- Con: the store layer does not know the manager's op semantics (rename, conversion, cascade labels, summaries); it would audit data migrations and imports that are deliberately recorded once per run; it duplicates the manager's records.
- Effort: L, with a worse record.

## Recommendation

Option C, delivered in three PRs:

1. Seam and SQL log (TKT-USQBMG6): split writer and reader capabilities; `sqlaudit` over `sqlexec` with postgres and sqlite adapters; hash chain (`hash = sha256(prev_hash, seq, record JSON)`, gapless `seq`); `rela audit verify`; `rela audit prune` that keeps an anchor so the chain still verifies; `rela audit import` for existing JSONL. On the database backends the SQL log replaces the JSONL sink. Appends outside a transaction are synchronous in their own transaction.
2. In-transaction recording (TKT-TU0VZ4Y): the entity manager writes its records on the transaction view; a failed append fails the write. Decision entity for the reversal of best-effort.
3. Read API and Changes tab (TKT-X1ZFO04): a reader capability declared at the data-entry call site, per-entity view gated like history reads, global view behind `audit:read`.

Tradeoffs accepted: an audit outage on postgres or sqlite blocks writes. The
chain detects editing or deleting rows by anyone who does not recompute it; an
attacker with write access to the database can rewrite the whole chain, so the
head hash printed by `verify` must be kept outside the database (or forwarded to
a SIEM, the follow-up ticket) to detect that.
