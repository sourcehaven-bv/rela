---
id: RES-03TUXO
type: research
title: 'SQLite store backend: capability scope, wiring, change feed, and the markdown trade-off'
summary: 'Target single-process desktop, not multi-process server: versioning is mandatory (it replaces git), enforce single-writer by advisory lock, and spike the WAL/BEGIN IMMEDIATE locking model as a go/no-go gate.'
status: done
---

## Problem

Should rela add a SQLite `store.Store` backend for single-server and desktop
deployments, and if so at what scope? TKT-TWIO11 settled the driver
(`modernc.org/sqlite`, pure Go, no CGO, FTS5/JSON1/R-tree verified present, all
six release targets cross-compile). Four questions remained:

1. **Capability scope** — minimal backend or full pgstore parity?
2. **Wiring** — a third `sqlite` build tag, or the default for `rela-desktop`?
3. **Change feed** — single-process only, or multi-process?
4. **The markdown trade-off** — what is actually lost versus fsstore?

The four are not independent. The central finding of this research is that
questions 2 and 3 collapse into one, and that answer then determines 1.

## Context

### The mandatory surface is smaller than the ticket assumed

`store.Store` is **26 methods** across 10 embedded interfaces. Three of them
(`GraphQuery`/`GraphCount`/`MatchingIDs`) are one-line delegations to
`internal/store/graphquerynaive`, whose `Reader` interface is deliberately
declared so any store satisfies it structurally with no adapter — fsstore's
entire implementation is 28 lines. So ~23 methods of genuine work.

Effort anchors, non-test lines: **memstore 1,068 · fsstore 2,996 · pgstore
6,169**. The ticket's "~10.5k as upper bound" counted test files and overstated
the ceiling by roughly 40%.

The conformance gate is real but cheap to adopt: `storetest.Capabilities` has
exactly **one** field (`Attachments`). There is no opt-out for headers, graph
query, watcher, validation or Tx — those run unconditionally. The memstore
template (`memstore/conformance_test.go`) is ~70 lines. Notably
`RunTxRollbackTests` — the strong Tx contract — is **not** in `RunAll`; pgstore
opts in separately and fs/mem deliberately do not, so a new backend may ship
with the weak contract and add rollback later.

### Search is a separate axis from the store

`search.Visible` wraps *any* `Searcher` and filters through a
`store.GraphQueryer`. A SQLite store paired with the existing bleve index is a
valid combination — **FTS5 is not required to ship**. Native
`search.VisibleSearcher` (FTS5 composed with visibility SQL) is an optimization,
not an entry requirement.

### Three `!postgres` no-ops silently encode "single-process"

This is the load-bearing constraint, and it is not a graceful degradation. Each
is justified in-tree by an assumption SQLite would violate the moment two
processes open the same database file:

- **`derivedschema_nosweep.go`** — "fsstore/memstore enforce `unique: true`
with the application-level check-then-write scan, which is correct for their
**single-process nature**". `store/derivedschema.go` names the hazard: "two
concurrent (especially cross-process) writers can both pass."
- **`kvuserstate`** — "a deployment running two servers over one project
directory will **silently lose snoozes** with this backend."
- **`stateKV`** — with an FSKV "an operator's logo upload lands on whichever
node served the POST and every other node keeps serving the old one, **with no
error anywhere**."

Inheriting the `!postgres` variants in a multi-process SQLite deployment is a
correctness regression with no error surface, not a reduced feature set.

### Three concrete-type assertions block a third smart backend

`appbuild` discovers pgstore capabilities by `st.(*pgstore.Store)`, not by
interface: `derivedschema_postgres.go:23`, `userstate_postgres.go:27`,
`versionsweep_postgres.go:42,93`. `StateKV` is likewise **not** a `store`
interface — it is a concrete `pgstore.StateKV` found via
`pgstore.StateStoreFor`. A SQLite backend cannot opt into the version sweep,
derived-schema reconciler or user-state without first widening these to
interfaces. That is a prerequisite refactor, easy to miss in an estimate.

### The change feed itself degrades cleanly

`dataentry.App.startStoreEventBridge` needs exactly one thing: `Subscribe`,
already mandatory in `store.Store`. If SQLite emits in-process events on its own
writes (the memstore/fsstore pattern), SSE works with zero dataentry changes.
Every consumer treats the feed as a hint — the contract is idempotent
re-snapshot, events may be dropped, and pgstore itself warns-and-continues if
its listener fails. MCP feature-detects `StartWatching` and logs a warning.

### The markdown trade-off is a whole feature, with precedent for losing it

`internal/git` is a full sync feature — status/fetch/sync/merge, conflict-file
listing, direct and PR modes — wired at `dataentry/app.go:968` behind
`git.IsRepo(paths.Root)`, with two API endpoints, a Vue store and a background
fetch loop. It carries **no build tag**, so it compiles into every build
including postgres.

It degrades gracefully: `gitOps == nil` returns `Available: false` and the UI
hides it. And the project has already made this trade once —
`docs/postgres-backend.md:194` frames content versioning as "the analogue of the
git history a filesystem project gets for free."

That links questions 1 and 4: **a SQLite backend that skips versioning loses git
and has nothing in its place.** For a server that may be acceptable. For a
desktop app whose users expect undo/history, it is not.

## Options

### Option A — Minimal backend, `sqlite` build tag, single-process

26 methods, naive graph query, bleve search, weak Tx, no optional capabilities.
Inherit all `!postgres` no-ops. A third `appbuild_sqlite.go` recipe plus
`rela-sqlite`/`rela-server-sqlite` binaries.

- **Pros** — smallest surface (~1.0–1.5k lines, memstore-shaped); no
prerequisite refactor; conformance passes day one.
- **Cons** — loses git with nothing replacing it; no versioning, no
`StateKV`/userstate/derived-schema; **single-process is unenforced**, so a
second process silently corrupts uniqueness and clobbers user state. Gains
little over fsstore beyond avoiding full-graph residency.
- **Effort** — S/M.

### Option B — Full pgstore parity

All 11 optional capabilities: versioning (9 methods + ~12 DTOs), purge,
`TypeWatermark`, `DerivedSchemaReconciler`, native FTS5 `VisibleSearcher`,
`StateKV`, `UserState`, `ManifestSince`, strong Tx with rollback, SQL pushdown.

- **Pros** — genuine middle tier; multi-process-safe; versioning replaces git;
could serve either side of the TKT-WE01O5 sync.
- **Cons** — approaches pgstore's 6.2k lines; requires the interface-widening
refactor; the purge guardrails are load-bearing and must be re-derived; large
surface to get right for a deployment tier that may not need all of it.
- **Effort** — XL.

### Option C — Desktop-first: versioning + safe-by-construction single-process

Take the capabilities that make SQLite *better than fsstore for a desktop app*,
and structurally prevent the multi-process hazards rather than documenting them.

Take: `HeaderReader`, `TypeWatermark`, the six versioning interfaces, strong Tx
(WAL + `BEGIN IMMEDIATE`), `StateKV`, `UserState`. Defer: native FTS5 search
(use bleve initially), `DerivedSchemaReconciler`, purge, `ManifestSince`, SQL
pushdown.

Enforce single-writer at open time — an advisory lock on the DB file so a second
process fails loudly instead of silently corrupting. That makes the inherited
`!postgres` uniqueness no-op *correct by construction* rather than correct by
assumption.

- **Pros** — versioning replaces git, so the markdown trade-off is honest and
matches existing postgres precedent; the single-process assumption becomes
enforced instead of hoped-for; ships without the FTS5 and derived-schema work;
needs the interface widening only for versioning/state, which is independently
useful.
- **Cons** — still needs the prerequisite refactor; versioning is the single
largest optional surface; deliberately forgoes multi-process, so it does not
replace pgstore for a shared server.
- **Effort** — L.

### Option D — Do nothing

- **Pros** — zero cost; fsstore already serves desktop, and git gives history
and sync free.
- **Cons** — leaves fsstore's full-graph residency and startup index rebuild
unaddressed at scale.
- **Effort** — none.

## Recommendation

**Option C, staged — but validate the concurrency model with a spike before
committing.**

The reasoning that decides it: questions 2 and 3 are the same question. SQLite's
value over fsstore is *not* multi-process (WAL gives concurrent readers but one
writer, and modernc's own tracker shows `SQLITE_BUSY` and `busy_timeout`
problems as its recurring complaint theme — upstream #232 and #192, both open).
Its value is a real indexed store for a *single* process that wants history
without a database server. That is the desktop app.

Once the target is single-process desktop, question 1 answers itself: versioning
is **not optional**, because it is what replaces git. And question 4 stops being
a loss — it becomes the same trade postgres already made, documented the same
way.

The multi-process hazards are then handled by construction. Rather than
inheriting three no-ops that silently assume single-process, take an advisory
lock at open so the assumption is enforced. A second process gets a clear error,
not silent uniqueness violations and clobbered snoozes.

Staging:

1. **Spike first** — prove the `Transactor` contract under WAL +
`busy_timeout` + `BEGIN IMMEDIATE`, running `RunTxStressTest` and the six fuzz
targets. Confirm bulk-write throughput on initial index build is acceptable
(modernc is ~1.5–1.9x slower than mattn on inserts, though that figure is
general driver overhead — FTS5 is un-benchmarked). **If the locking model does
not hold cleanly, stop and reconsider** — that is the documented risk area for
this driver.
2. **Widen the three `*pgstore.Store` assertions to interfaces.** Independently
valuable and a prerequisite either way.
3. **Minimal backend + conformance**: 26 methods, naive graph query, bleve
search, `HeaderReader`, single-writer lock.
4. **Versioning + `StateKV`/`UserState`** — the tier that earns the git trade.
5. **Later, if warranted**: FTS5 native search, SQL pushdown,
`DerivedSchemaReconciler`, `ManifestSince`.

Trade-offs accepted: no multi-process (deliberate, enforced); no
git-diffable/hand-editable files on this backend; ~1.5–1.9x slower bulk writes
than a CGO driver; no custom FTS5 tokenizers ever.

Stages 1–3 are independently useful and stage 1 is a genuine go/no-go gate, so
the commitment is incremental rather than all-or-nothing.
