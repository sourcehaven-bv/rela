---
id: RES-Z1SJ5
type: research
title: 'Store-based transactions: per-backend write serialization/atomicity as a store capability (replaces dataentry writeMu)'
summary: Recommend an optional store.Transactor capability (fs = write mutex, pg = BEGIN + xact advisory lock) wrapped around each entitymanager intent; deletes dataentry writeMu, gains cross-process serialization and intent atomicity on postgres; run as its own arc after M5.4.
status: done
---

## Problem

`dataentry.App.writeMu` is a process-local mutex that serializes the data-entry
HTTP mutation handlers against each other — a poor-man's transaction system. It
provides isolation-by-serialization for one process and one entry point, and
nothing else: no atomicity (an action script failing at write 3 of 5 leaves 1–2
committed), no isolation from CLI/MCP/scheduler writers, nothing cross-process
on postgres.

This is evolutionary residue: rela grew from markdown-on-disk (where a global
write lock *is* the strongest consistency the backend can offer) to a dual
fs/postgres deployment. The strongest backend now runs under the weakest
backend's concurrency model, and per-process at that.

The question: should write serialization/atomicity become a **store capability
with per-backend meaning** — fsstore implementing it as its write mutex, pgstore
as native transactions — replacing `writeMu`? This also settles the open M5.4
question in [[TKT-R68TV8]] ("mutex behind the store vs. App-level
pointer-shared"): both listed options assumed the mutex survives; this is the
third option where it dissolves.

## Context (codebase survey, 2026-07-16)

**What one entitymanager "intent" actually is.** The write path is already
multi-write and non-atomic:

- `CreateEntity` = validate → `Store.CreateEntity` → audit → automation →
possibly a **second** `Store.UpdateEntity` (manager.go:472) → cascade.
- `DeleteEntity` = version capture (entity + per-relation) **before** the store
delete (manager.go:694) → `Store.DeleteEntity` → audit. The code itself notes
"the store delete is still not transactional with this capture; strict atomicity
is a future hardening" (manager.go:689–693).
- Cascade side-effects write **directly to the store** via `cascadeHost`
(cascadehost.go:54,83,108,159), bypassing Manager to avoid re-cascading.
- ~11 store-write call sites across the package, all against the single
`Deps.Store` field — no per-call store threading exists today.

**Store capability precedent.** `store.Store` (9 embedded interfaces) has no
transaction/batch method; its doc promises only per-write atomicity ("Writes
serialize internally — callers do not need external locking", store.go:161–166).
Seven optional capabilities already exist as type-asserted interfaces
(`Formatter`, `VersionWriter`, `HistoryReader`, `RelationVersionWriter`,
`RelationHistoryReader`, `VersionPurger`, `RelationVersionPurger`) — the exact
pattern a `Transactor` would follow.

**Event timing is already half-way there.** pgstore wraps *every* write in a tx
solely so `pg_notify` commits atomically with the row, and fans out in-process
observers/subscribers **after commit** (pgstore.go:196–197, entity.go:233–242).
fsstore/memstore emit synchronously under their write lock. So "events fire
post-commit" is already pgstore's model; a Tx capability generalizes rather than
invents it.

**Advisory locks are established idiom in pgstore.** Migrate uses a xact-scoped
lock (migrate.go:56); sweep and purge share a session-scoped lock key for mutual
exclusion (sweep.go:19, purge.go:49). A write-serializing
`pg_advisory_xact_lock` inside a `Tx` is a natural extension.

**Who bypasses writeMu today.** MCP tools, CLI commands, and the scheduler all
write via entitymanager with **no serialization at all** (grep: no mutex in
`internal/mcp`, `internal/cli`, scheduler). On postgres there is **no
cross-process serialization of ordinary writes** — only per-statement/per-tx
atomicity + ON CONFLICT. writeMu protects strictly less than it appears to. Note
also that the writer entry points rarely share a process (MCP, CLI, scheduler
are separate binaries), which matters when weighing in-process-only fixes.

**The long-transaction hazard is real but already bounded.** Action scripts hold
writeMu for the whole Lua run (actions.go:80), and write-time Lua can call the
AI provider over HTTP (lua/ai.go:48–50) — slow external I/O under the lock. The
5s `actionTimeout` exists precisely because of this (actions.go:23–26). Note: on
multi-process postgres, the script-level critical section is already illusory —
other processes' writes interleave freely today.

**Constraints.** Root CLAUDE.md: "No repository or transaction abstractions —
the old `repo` and `tx` layers are gone, do not reintroduce equivalents."
Adopting any store-Tx option requires a decision entity amending/clarifying this
rule (the removed layer was generic indirection *above* the store; a per-backend
capability *on* the store is a different animal — but that argument must win a
design review, not be assumed). Also: dataentry philosophy (DEC-HWZHA)
deliberately tolerates temporarily invalid data; transactions here are about not
*interleaving/losing* writes, not about rejecting invalid state.

## Anomaly classes (the yardstick)

Options are judged against the five concrete anomaly classes the current design
permits:

1. **Check-then-act races** — e.g. `checkUniqueProperties` →
`Store.CreateEntity` (core.go:89–128): two concurrent creates both pass the
uniqueness check, both write. writeMu prevents this only between dataentry
handlers in one process; CLI-vs-server or server-vs-server races it today.
2. **Lost updates** — two writers read the same snapshot, both write; last
write wins silently. Serialization does NOT fix this (serialized ≠ not-lost);
only conflict detection does.
3. **Partial intents** — error/crash mid-intent leaves fragments: version
capture without its delete, create without its automation property-write, audit
row for work that half-happened.
4. **Interleaved cascades** — two intents' automation/cascade write sequences
interleave, producing orderings no serial execution could produce (duplicate
side-effect entities, `if_exists: skip` racing itself).
5. **Interleaved scripts** — an action script's multi-write sequence
interleaves with other writers. In-process, writeMu prevents this today;
cross-process on postgres it is already unprotected.

## Options

### Option A — Status quo: keep the mutex app-level, move it with the write nucleus

**Mechanics.** M5.4 extracts the write handlers; the new struct owns `writeMu`;
sync/attachment/action/webhook handlers keep face-sharing it. No semantic
change of any kind.

**Fixes:** nothing new. Preserves in-process protection against 1, 4, 5 for
dataentry handlers only.

**Leaves open:** 1 and 4 cross-process and for CLI/MCP/scheduler; 2 always; 3
always.

**Introduces:** no new risk.

**Pick if:** rela's consistency posture — "tolerate temporarily invalid data,
analyze later" — is judged to extend to *interleaving* anomalies, not just
*invalid-state* anomalies. That is a defensible position for fs-first,
mostly-single-writer deployments: `analyze unique` (FEAT-3DUA6) can find
duplicate-unique violations after the fact, and markdown-on-disk users already
live with hand-edit races.

**Don't pick if** postgres multi-writer deployment is a real product direction.
The anomaly classes above are not "temporarily invalid, converge later" — a lost
update or duplicated cascade is *silently wrong*, and `analyze_*` can detect
some (duplicates) but recover none (lost writes). Also leaves the manager's own
documented atomicity TODO unaddressed indefinitely.

### Option E — Move the mutex into entitymanager (in-process, no store change)

**Mechanics.** Manager gains a `sync.Mutex`; every intent
(Create/Update/Delete/Rename, relation ops) locks it. dataentry deletes writeMu
except a slim script-scoped mutex for action scripts (script writes go through
the Mutator per-call, so whole-script exclusion needs its own lock).

**Fixes:** 1 and 4 for *all entry points within one process* — the natural
"right place" argument, since the manager owns the check-then-act sequences.

**Why it disappoints in practice:** the survey shows the writer entry points
almost never share a process — MCP, CLI, and the scheduler are separate
binaries. So E mostly relocates the mutex without extending its protection: in
an fs deployment the server process was already the only in-process multi-writer
(dataentry handlers), which writeMu covers; cross-process races (the real gap)
remain untouched on both backends. It also adds a lock-ordering constraint
between the manager's mutex and each store's internal mutex.

**Pick if:** you want a cheap, honest staging step toward Option B — the "wrap
each intent" boundary work is identical, and swapping the mutex body for
`store.Tx(...)` later is mechanical.

**Don't pick as an end-state:** it costs the migration churn of B's boundary
work while delivering approximately Option A's guarantees.

### Option B — `store.Transactor` capability (recommended)

**Mechanics.**

```go
type Transactor interface {
    Tx(ctx context.Context, fn func(Store) error) error
}
```

Contract v1: writes inside `fn` are isolated from every other transaction —
cross-process where the backend can see them — observer/subscriber events are
delivered post-commit, and errors roll back where the backend supports it.
entitymanager wraps each intent in one `Tx`; cascadeHost receives the tx-bound
store, so cascade writes join the intent's transaction.

Per backend:

| | fsstore/memstore | pgstore |
|---|---|---|
| Isolation | internal write mutex held across `fn` (process-local — matches fs's single-process reality) | `BEGIN` + `pg_advisory_xact_lock(<write key>)` — deployment-wide |
| Atomicity | none in v1 (error leaves partials, same as today); v2 option: pre-image journal for error-path rollback | full rollback on error |
| Events | buffered, emitted after `fn` returns nil | already post-commit; buffer per-write events until Tx commit |
| Crash safety | none (no WAL — don't pretend) | full (it's postgres) |

**Fixes:** 1, 3, 4 fully on postgres, cross-process included; 1 and 4 in-process
on fs (3 on fs only with the v2 journal). Directly delivers the `manager.go:689`
"future hardening." Every entry point inherits the semantics because the
boundary is entitymanager, not dataentry.

**Deliberately does NOT fix:** 2 (lost updates — see Option C) and 5
(whole-script atomicity — see hazards below).

**Design sub-decisions the review must settle:**

- **Optional capability vs. interface member.** The optional-capability
pattern fits features only some backends can implement — but *every* in-tree
backend can implement `Tx` (fs trivially, via its mutex). If entitymanager
*requires* Tx for correctness, "optional" is a polite lie that forces a dual
code path (with-Tx / fallback) through the most safety-critical package.
Leaning: put `Tx` on `store.Store` proper, enforced by the storetest conformance
suite; the CLAUDE.md amendment then covers "the store interface grew a
transaction method," which is more honest than smuggling it in as a capability.
- **fs re-entrancy.** fsstore's write methods take its internal `mu.Lock`;
if `Tx` holds that same lock, the first write inside `fn` deadlocks. The
tx-bound store view fs hands to `fn` must skip locking (a `locked` variant of
the store, not a re-entrant mutex hack). Mechanical, but must be designed and
race-tested, not improvised.
- **Lock granularity on pg.** v1 = one global write key: zero deadlock risk,
no retry machinery, exactly the fs model but cross-process correct. A per-entity
hashed key would allow concurrent non-conflicting intents but reintroduces
deadlock potential the moment an intent touches multiple ids
(delete-with-cascade always does). Granularity is a later optimization *inside*
pgstore, invisible to callers.
- **Nesting = joining.** No nested-Tx API. Being "in" a transaction means
holding the tx-bound store; cascadeHost and the version writers take whatever
store they're handed. This keeps the manager's ~11 write sites a threading
change, not a semantic one.

**Introduces (honest risk list):**

- One connection pinned per Tx on pg, and one slow `fn` stalls the whole
deployment's writers (global advisory lock). Bounded today by tight timeouts,
but it makes "no external I/O inside an intent" a *rule* rather than a
preference — which is why whole scripts stay out of Tx.
- Event buffering is a behavior change: the search indexer and SSE bridge see
an intent's events at commit, slightly later than today. More correct (never
index a rolled-back write) but has test surface.
- Audit moves to post-commit — same "after the durable write" guarantee,
correctly generalized; an audit row must never exist for a rolled-back write.
(Denied-write audit rows, emitted before any write, are unaffected.)
- Plumbing through entitymanager is the highest-risk work in the tree; must
be gated on the race suite + storetest.

**Pick if:** postgres multi-writer is real, and the goal is one consistency
story owned by the layer that differs per backend. This is the only option that
fixes cross-process anomalies *and* intent atomicity, and it dissolves writeMu
rather than relocating it.

**Don't pick if:** rela's postgres story stays effectively single-writer per
deployment — then B is over-engineering and A is honest.

### Option C — Optimistic concurrency (revision/If-Match CAS)

**Mechanics.** Every entity carries a revision (pg: column, bumped per write;
fs: harder — a frontmatter field is user-editable and hand-edits would fight it;
realistic fs answer is content-hash-as-revision). Write APIs accept an expected
revision; mismatch → 409; SPA grows conflict UX (reload/merge prompt); CLI/MCP
grow a `--if-revision` / optimistic-retry story.

**Fixes:** 2 — the *only* option that does. Lost updates are invisible to
serialization: writeMu, Option B, and Option E all admit "user A and user B both
read v1, B saves, A saves, B's edit silently gone."

**Why it's not the answer to *this* question:** it provides no multi-write
atomicity (a CAS guards one write, not an intent with automation + cascade), so
it doesn't replace writeMu or fix classes 1/3/4 — automations and cascades don't
have a "user" to bounce a 409 to. Its blast radius is product-wide (API + SPA +
CLI + MCP + docs + conflict UX), i.e. XL and mostly UX work, not storage work.

**Relationship to B:** complementary, not competing. B gives atomic, serialized
intents; C layered on top gives conflict *detection* for human edits. B's
pgstore can even adopt CAS internally later without caller changes. Sequencing C
first would mean building conflict UX on top of non-atomic intents — the 409
would fire against states that are themselves torn.

**Pick if/when:** multi-user editing of the same entities becomes a real
complaint. **Don't pick now:** wrong tool for the writeMu question.

### Option D — Advisory lock inside the existing per-write txs only

**Mechanics.** Add `pg_advisory_xact_lock` to pgstore's existing per-write
transactions. No interface change, no manager change, ~20 lines.

**Fixes:** nothing observable. The anomalies live *between* the statements of
one intent (check→create, capture→delete, write→cascade); D serializes only
*within* single statements, which postgres row locks already effectively do.
writeMu survives untouched.

**Why it's listed:** it's the tempting "cheap compromise" someone will suggest
in review, and it should be pre-refuted: it adds lock traffic on every write
while closing none of the five anomaly classes. **Don't pick.**

## Comparison matrix

| Anomaly / property | A (status quo) | E (manager mutex) | B (store Tx) | C (CAS) | D (advisory only) |
|---|---|---|---|---|---|
| 1. check-then-act races | dataentry, in-process | all entry points, in-process | **all writers; cross-process on pg** | partially (single-write CAS only) | no |
| 2. lost updates | no | no | no | **yes** | no |
| 3. partial intents | no | no | **pg: yes; fs: v2 journal** | no | no |
| 4. interleaved cascades | dataentry, in-process | in-process | **yes; cross-process on pg** | no | no |
| 5. script critical section | in-process | in-process (slim mutex) | dropped by design (slim mutex optional) | no | in-process (writeMu stays) |
| writeMu fate | relocated | deleted (+slim script mutex) | **deleted** (+optional slim script mutex) | unchanged | unchanged |
| CLAUDE.md rule amendment | no | no | **yes (decision entity)** | no | no |
| New failure modes | none | lock ordering | long-tx stalls, event timing, plumbing risk | conflict UX debt | lock traffic |
| Effort | nil | M | **L** | XL | S |

## Recommendation

**Option B**, sequenced as its own arc *after* M5.4 lands conservatively (Option
A is the correct *interim* state, not the end state; Option E is only worth
doing as B's first commit, not as a destination):

1. M5.4 proceeds now with the mutex moving to the write-nucleus struct,
semantics untouched (refactors must not change concurrency behavior).
2. A decision entity records the amendment to the "no transaction
abstractions" rule — and settles the optional-capability vs. interface-member
question (leaning: interface member, since every backend implements it and the
manager depends on it for correctness).
3. Implementation order: storetest contract for `Tx` (incl. fs re-entrancy
and post-commit event assertions) → fsstore/memstore → pgstore (`BEGIN` + global
xact advisory lock + buffered events) → entitymanager wraps intents (cascadeHost
takes the tx-bound store) → delete `writeMu`, with an explicit keep/drop
decision on the slim script mutex.
4. Explicitly out of scope for v1: fs rollback journal, optimistic
concurrency (Option C), per-entity lock granularity, any retry-on-conflict
machinery.

**Tradeoffs accepted:** fs gets isolation without rollback (documented asymmetry
— honest, since fs can't promise crash atomicity anyway); deployment-wide
advisory lock means one slow in-tx operation stalls all pg writers (mitigated by
keeping Lua scripts out of whole-script txs and by the existing tight timeouts);
lost updates (class 2) remain — that is Option C's job, deliberately deferred;
the entitymanager plumbing is nontrivial and must be guarded by the
race-detector suite and storetest conformance.
