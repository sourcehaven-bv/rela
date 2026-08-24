---
id: TKT-TWIO11
type: ticket
title: Investigate SQLite as a third store backend for single-server and desktop deployments
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

Evaluate an embedded SQLite `store.Store` backend that sits conceptually between
`fsstore` (single-user, markdown-on-disk, no server) and `pgstore`
(multi-process, multi-tenant, server-managed). Target deployments: a single
`rela-server` instance, and the `rela-desktop` app.

Scope of this ticket is **investigation and a recommendation** — a research
entity plus a decision — not implementation.

## Why

rela has two store backends today, with a large gap between them:

- **`fsstore`** — markdown files on disk, git-friendly, single-writer by nature, in-process events only. No SQL pushdown: `GraphQuery` runs through `graphquerynaive`, search is an in-memory bleve index rebuilt at startup, and the whole graph is effectively resident.
- **`pgstore`** — ~10.5k lines, native SQL pushdown, `LISTEN/NOTIFY` cross-process change feed, content + relation versioning, `state_kv`, derived-schema reconciler, version sweep, purge. Requires operating a PostgreSQL server.

A single-server or desktop deployment wants pgstore's *properties* (indexed
queries, no full-graph residency, real transactions, versioning) without
pgstore's *operational cost* (a separate database process, a DSN, migrations
against a shared cluster). That is exactly the niche SQLite occupies.

## What to investigate

**1. Conformance surface.** What must a new backend actually implement?
`store.Store` = `EntityReader + EntityWriter + RelationReader + RelationWriter +
GraphQueryer + AttachmentManager + Watcher + Lifecycle + Freshness +
Transactor`, and it must pass `internal/store/storetest` (`RunAll` + fuzz —
~4.7k lines covering graphquery, relation, entity, attachment, pagination, tx,
watcher, stress, differential). Establish the minimum viable backend versus the
full-fidelity one.

**2. Which optional capabilities to take.** These are type-asserted, not part of
`store.Store` — each is opt-in and each is a decision point: `HistoryReader` /
`VersionWriter` / `RelationHistoryReader` / `RelationVersionWriter` (content
versioning), `VersionPurger` / `RelationVersionPurger`, `StateKV`,
`store.Formatter`, the derived-schema reconciler, and a native
`search.VisibleSearcher` (SQLite FTS5 as the analogue of `pg_trgm` + tsvector —
must pass `storetest.RunVisibleSearchTests`).

**3. CGO / driver — SETTLED: `modernc.org/sqlite` (pure Go, no CGO).** Decided
2026-08-22. Verified empirically against `modernc.org/sqlite v1.57.0`, not
inferred:

- **FTS5 is compiled in unconditionally** — the `SQLITE_ENABLE_FTS5` flag is
baked into the transpiled C, so there is no build tag, no opt-out, and no way to
ship a binary without it. Present since before v1.4.0 (2020). Verified with
`CGO_ENABLED=0`: FTS5 table creation, `unicode61`/`porter`/`trigram` tokenizers,
`bm25()` + `ORDER BY rank`, `snippet()`/`highlight()`, and
external-content/contentless tables all work. `trigram` is the `pg_trgm`
analogue.
- **JSON1 and R-tree are also compiled in** (`json_extract` verified), which
matters for pushing property predicates into SQL. `STAT4` too.
- **Cross-compiles to all six release targets with `CGO_ENABLED=0`**:
linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/{amd64,arm64}. The GoReleaser
matrix therefore stays free — the sqlite variant is just another `CGO_ENABLED=0`
entry alongside the existing five.

Consequences of NOT taking `mattn/go-sqlite3`, accepted knowingly:

- **Bulk writes are ~1.5–1.9x slower** than mattn (consistent direction across
two independent 2026 benchmark suites; reads near-par, better under
concurrency). Caveat: neither suite benchmarks FTS5, so that figure is general
driver overhead, not a measured FTS5 number. Mitigable with a single
transaction, WAL, `synchronous=NORMAL`, prepared statements. Confirm during the
spike that initial index build is acceptable.
- **Custom FTS5 tokenizers are impossible** — the C symbols exist in the
transpiled code but the driver exposes no registration API (mattn needs CGO
callbacks; modernc is a hard no). Workaround is to pre-tokenize in Go into a
contentless/external-content table. Only bites if the four built-in tokenizers
prove insufficient.
- **FTS3/FTS4 are absent.** Irrelevant for new work; noted in case a legacy
schema ever has to be read.

Follow-ups this creates for the spike:

- **Pin `modernc.org/libc` to the exact version in `modernc.org/sqlite`'s own
`go.mod`** — a documented fragility (upstream #177); a transitive bump by
another module can break the build. Warrants a `go.mod` comment.
- **`PRAGMA optimize`** on close/periodically. SQLite 3.51.0 changed the query
planner such that an FTS5 table joined against an ordinary table can be 10–40x
slower without statistics (confirmed upstream by Hipp, reproduced in pure C). We
will be doing exactly that join. Driver-independent — it hits mattn identically
— so it is not a selection criterion, but it is a real operational requirement.
- **`windows/arm64` reuses amd64-generated code** rather than being separately
transpiled, and there is an open WAL-startup crash report on Windows (upstream
  #221). Builds fine; treat as the least-exercised target and verify before
shipping a Windows arm64 archive.

Fallback if modernc is ever abandoned for mattn: CGO *can* be confined to the
sqlite build. Verified — `//go:build sqlite` plus `CGO_ENABLED=1` on that one
GoReleaser entry leaves the existing five binaries untouched at `CGO_ENABLED=0`.
The sharp edge is that `CGO_ENABLED` is an env var, not a build tag, so
`-tags=sqlite` with CGO off fails as a confusing "build constraints exclude all
Go files"; a `//go:build sqlite && !cgo` file emitting a clear compile-time
error would be required. Not needed under the current decision.

**4. Concurrency model — now the highest-risk open question.** SQLite is
single-writer even in WAL mode. Map that onto the `Transactor` contract
(DEC-8UIL0): WAL + `busy_timeout` gives cross-process *reader* concurrency and
serialized writers, landing strictly between fsstore's in-process mutex and
pgstore's advisory-lock + native transaction. Does it need `BEGIN IMMEDIATE` to
avoid lock-upgrade deadlocks?

Locking — not FTS5 — is the recurring complaint theme in modernc's issue
tracker, so this is what a spike must actually exercise rather than assume:
frequent `SQLITE_BUSY` after migrating from mattn (upstream #232, open) and
`busy_timeout` not respected with `_txlock=immediate` (upstream #192, open). A
background indexer writing concurrently with request-path writes is exactly the
shape those reports describe.

**5. Change feed.** pgstore has `LISTEN/NOTIFY`; SQLite has no equivalent. For a
genuinely single-process deployment, in-process fan-out is sufficient and
honest. Decide whether multi-process is in scope at all — if it is, the options
(update hook, polling a sequence, a sidecar) all carry real cost and this
becomes a much larger ticket.

**6. Build-tag placement.** The existing pattern is 17 `//go:build postgres`
files. Determine whether a `sqlite` tag replicates that (a third
`appbuild_sqlite.go` recipe, `mcp_wiring_sqlite.go`, `cli/db_sqlite.go`, a third
`rela-server-sqlite` binary, a third CI job) or whether SQLite should instead be
the **default** backend for `rela-desktop`. Note the CLAUDE.md constraint: the
postgres build must not link bleve and the default build must not link pgx — a
sqlite build carries the same isolation obligation, asserted via `go list
-deps`.

**7. Config-on-disk invariant.** `schema.yaml`, `acl.yaml`, `templates/`,
`scripts/` stay on the filesystem even on the postgres build. Confirm SQLite
inherits that: it backs entities/relations/attachments/search/state, not
operator-authored config. A `--project` dir is still required.

**8. What is lost versus fsstore.** fsstore's markdown files are the
git-diffable, hand-editable, grep-able source of truth — a real product
property, not an implementation detail. A SQLite backend gives that up. Is there
an export/sync story (cf. `TKT-WE01O5`, two-way fsstore↔pgstore sync), or is
that an accepted trade for the deployments this targets?

## Out of scope

- Implementation. This ticket produces a research entity and a decision.
- Multi-process SQLite coordination beyond assessing feasibility.
- Migration tooling between backends.

## Deliverables

- A `research` entity (via `/research`) covering the still-open options with pros/cons/effort: minimal backend (no versioning) vs full-fidelity; build-tag vs desktop-default wiring; multi-process in or out. The driver question is closed (see section 3) — carry the finding forward, do not re-litigate it.
- A recommendation, and a `decision` entity if the recommendation is to proceed.
- A rough effort estimate for the implementation ticket, anchored against pgstore's ~10.5k lines as the upper bound.

## Acceptance criteria

**ALL MET.** Outcome: **GO** — see DEC-LFSYNY. Evidence: RES-03TUXO (survey),
`internal/store/sqlitespike/RESULTS.md` (measurements, branch
`spike/sqlite-tx-TKT-TWIO11`).

1. ~~Enumerate required vs optional methods with take/skip rationale.~~ **MET** —
26 mandatory methods across 10 embedded interfaces (3 of them one-line
delegations to `graphquerynaive`), plus 11 optional type-asserted capabilities.
Effort anchors, non-test lines: memstore 1,068 / fsstore 2,996 / pgstore 6,169.
The original "~10.5k upper bound" counted test files and overstated by ~40%.
2. ~~CGO / driver recommendation.~~ **MET 2026-08-22** — `modernc.org/sqlite`,
pure Go, `CGO_ENABLED=0`, all six release targets cross-compile, FTS5/JSON1/
R-tree verified compiled in. See section 3.
3. ~~`Transactor` semantics stated precisely alongside fs and pg.~~ **MET —
measured, not assumed.** SQLite sits at **pgstore's tier**: real rollback and
post-commit-only events (`RunTxRollbackTests` passes), giving up only
cross-process serialization. 30s soak: 31,106 counter commits, 22,036 pair txs,
no deadlock. `BEGIN IMMEDIATE` is load-bearing (the deferred control arm fails).
Not covered by that soak, and stated as such: attachments in a Tx,
`RenameEntity`, `Close` under an open Tx.
4. ~~Change-feed decision explicit.~~ **MET — single-process only, enforced by
an advisory lock rather than documented.** In-process fan-out satisfies the SSE
bridge, which needs only `Subscribe`. Multi-process is rejected: `unique:` is an
**untransacted** `ListEntities` scan (`entitymanager/unique.go:82`, zero `.Tx(`
sites in the package), so a multi-process SQLite backend inheriting the
`!postgres` no-op would have no uniqueness backstop at all.
5. ~~Wiring approach chosen and checked against the backend-isolation CI
assertion.~~ **MET** — a `sqlite` build tag mirroring the 17-file `postgres`
pattern, NOT the `rela-desktop` default (that is a separate call once the
backend exists). CI gains the matching `go list -deps` assertions; the default
build must not link the sqlite driver. Detailed in TKT-G91TBK.
6. ~~Go/no-go with an effort estimate.~~ **MET — GO**, staged:
TKT-415WA7 (`m`, prerequisite interface widening) → TKT-G91TBK (`l`,
conformance-passing store) → stage 4 versioning + state, **unestimated until
scoped**.

## Outcome

- **Decision**: DEC-LFSYNY (accepted)
- **Research**: RES-03TUXO
- **Follow-ups**: TKT-415WA7 (ready), TKT-G91TBK (backlog, depends on it)
- **Spike branch**: `spike/sqlite-tx-TKT-TWIO11` — never merge; kept for the
measurements and the reproduction harness.

One acceptance-criterion caveat carried forward honestly: **arm F (WAL on a
network/sync filesystem) was not run** — no network storage available. It is
recorded as an accepted, *unmeasured* limitation, with a concrete mitigation in
TKT-G91TBK (refuse or warn at startup when `journal_mode` is not `wal`).
