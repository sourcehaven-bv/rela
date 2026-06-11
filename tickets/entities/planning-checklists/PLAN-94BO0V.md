---
id: PLAN-94BO0V
type: planning-checklist
title: 'Planning: Multi-writer support for pgstore (cross-process change feed)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Make pgstore correct under multiple concurrent writers AND make it demonstrable
end-to-end via the data-entry live-reload (scope (b)). Three layers:

IN scope:
1. **pgstore change feed (producer + listener).** On every committed write,
`pg_notify(<channel>, '<origin>:<kind>:<op>:<id>...')` where `<channel>` is
SCHEMA-SCOPED (see RR-CPZGAK) and `<origin>` is a per-store nonce (RR-97VOON). A
listener goroutine holds a dedicated connection, receives notifications, skips
its own (origin == self), parses the rest into `store.Event`s, and delivers them
to the store's existing in-process `Subscribe()` fan-out — so REMOTE writes flow
through the same channel as local writes.
2. **Durability backstop (seq catch-up).** NOTIFY has no replay, so on listener
startup / reconnect / a slow safety ticker, run `WHERE seq > $lastSeen` over the
3 tables, emit a store.Event per row, advance the watermark to `max(seq) -
OVERLAP` (overlap window + idempotent re-snapshot; xid8 deferred).
3. **Consumer wiring (data-entry SSE), ENTITIES ONLY.** A store-event -> SSE
bridge in dataentry: subscribe to `store.Subscribe()` and map ENTITY
create/update/delete -> `broadcastEntityEvent`. Relations/attachments are NOT in
the live feed (they aren't today either — RR-GNS360). This makes a remote ENTITY
write appear live in a browser on another server; it also fixes the pre-existing
gap where fsstore emits entity events nobody consumed.

OUT of scope:
- xid8 column / exact-once ordering (documented upgrade path only).
- Changing the `store.Watcher` interface signature.
- **Relation/attachment cross-process live-reload** — relations emit store
events but NO dataentry code broadcasts them today (local or remote); matching
status quo, we do NOT bridge them. Attachments emit no store events at all.
Documented as a conscious decision (RR-GNS360), separate enhancement if wanted.
- Multi-writer for fsstore/memstore; HA/failover/replication; pool tuning.
- MCP cross-process notifications (re-reads per request; note in docs).

**De-dup decision:** the store-event bridge becomes the SINGLE source for entity
SSE broadcasts. Remove ONLY the 3 inline entity broadcasts at api_v1.go:592
(created), 905 (updated, gated on `entityChanged`), 963 (deleted) — RR-MZOKST.
**Do NOT remove** handlers_git.go:104 (git status), watcher.go:115 (config
refresh), watcher.go:164 (git fetch) — those have no store write and the bridge
doesn't cover them. The local in-process emit (instant) still fires for local
writes; the bridge sees those same events and broadcasts once (the removed
inline calls were the duplicate). The listener does NOT re-emit local writes
(origin filter), so no triple-broadcast.

**Acceptance Criteria:**

1. **Cross-process propagation.** Two pgstore instances A,B on one schema; a
committed entity write by A is delivered to a `Subscribe()` channel in B within
~1s. **Test:** two `pgstore.Open` on the same test schema, Subscribe on B,
CreateEntity on A, assert B receives the matching store.Event.
2. **Catch-up recovers missed events.** Write on A while B's listener is stopped;
start B's listener; B emits the event from catch-up. **Test:** as described.
3. **No skew miss under interleaved commits.** Concurrent out-of-commit-order
transactions are all eventually delivered (overlap window + idempotent apply).
**Test:** interleaved writers; every id reaches the subscriber (dups allowed).
4. **SSE entity live-reload across processes.** A browser on server B's
`/api/v1/_events` receives `entity:created/updated/deleted` when server A writes
an entity via its API. **Test:** two dataentry apps on one DB; SSE-subscribe on
B; POST create on A; assert B's SSE emits it. Plus: a LOCAL entity write
broadcasts EXACTLY ONCE (no dup from inline+bridge, no triple from listener
self-echo). NOTE: relations/attachments explicitly NOT asserted in the live feed
(out of scope — unchanged from today).
5. **Single-process unchanged.** storetest.RunAll green; a local write's event is
observed immediately WITHOUT depending on the listener (in-process emit stays).
6. **No schema migration** beyond TKT-M8400 (NOTIFY + queries only; no new file).
7. **Test isolation under parallelism (NEW, from RR-CPZGAK).** Two multi-writer
tests on different schemas in the same DB do NOT cross-talk. **Test:** run the
suite with `-p`/parallel; assert a listener on schema X never receives schema
Y's notifications (schema-scoped channel).

## Research

- [x] Ran `/research` -> RES-4ZMBTA (LISTEN/NOTIFY vs poll vs CDC; chose A).
- [x] Libraries: pgx/v5 v5.9.2 (existing) has `Conn.WaitForNotification(ctx)`,
`pgxpool.Pool.Acquire`, `pgconn.Notification{PID,Channel,Payload}` (verified in
module cache). `pgxlisten` REJECTED (pre-v1/untagged) — hand-roll reconnect.
- [x] Codebase patterns verified file:line (see Approach).
- [x] Reviewed concepts: store-backends.

**Research Doc:** RES-4ZMBTA

## Approach

- [x] Technical approach chosen and documented
- [x] Builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified (pgx/v5 existing; no new module)

**Technical Approach (revised per design review):**

1. **Producer — wrap ALL writes in a transaction (RR-ITQN87).** Today only
DeleteEntity/RenameEntity/AttachFile use `tx := s.db.Begin()`; the other 5
(CreateEntity entity.go:219, UpdateEntity :249, CreateRelation relation.go:151,
UpdateRelation :178, DeleteRelation :193) are single-statement autocommit, so
there's no tx to attach pg_notify to. **Wrap those 5 in an explicit tx** so the
write + `notify(ctx, tx, channel, payload)` commit atomically (no
notify-on-rollback; trivial overhead). One `notify` helper used by every write
path, called inside the tx before commit. Payload = `<origin>:<kind>:<op>:<id>`
(relation: `...:<from>|<reltype>|<to>`). Delimiter must avoid chars validateID
permits (IDs reject `--`, `/`, `\`, control chars) — use a delimiter validateID
forbids, or length-prefix.

2. **Channel scoping (RR-CPZGAK).** Channel = `rela_changed_<schema>` (LISTEN is
database-global; per-test schemas would cross-talk on a constant channel).
Resolve the active schema once at Open (search_path's first entry / a `SELECT
current_schema()`), compute the channel, and pass it to BOTH the producer helper
and the listener so they match. Fits the 63-byte identifier limit.

3. **Per-store origin nonce (RR-97VOON).** Add `originID string` to Store,
generated in New/Open (crypto/rand). Producer embeds it; listener skips
notifications whose origin == s.originID (our own write — already emitted
in-process). Notification.PID is the listener backend's, useless for self-id —
nonce is required.

4. **Listener (new listener.go).** Started by the postgres `Open` (it has the
`*pgxpool.Pool`). Holds a dedicated STANDALONE `pgx.Conn` (from the DSN, not the
pool — don't shrink the query pool). Runs `LISTEN <channel>`, an initial
catch-up, then loops `WaitForNotification(ctxWithTimeout~30s)`: notification ->
skip-if-self -> parse -> `s.emit(ev)`; timeout -> catch-up; error -> reconnect
   + re-LISTEN + catch-up. Owned by the store: a `listener` field + its cancel;
`Store.Close()` cancels the listener ctx and closes its conn BEFORE closing
subscriber channels, and this happens before appbuild's poolCloser closes the
pool (the listener uses its own conn, not the pool, so no use-after-close, but
order Close to stop the goroutine first). Connection seam: pass the DSN (or a
standalone-conn factory) into the postgres `Open`; keep `New(DBTX)` listener-
free for the conformance tests that don't want a feed.

5. **Listener-unavailable behavior (was open; DECIDE):** if the listener can't
establish its connection at Open, **degrade with a loud warning** — the store
   + local events still work; cross-process events are just unavailable. Matches
the "search index unavailable -> error-Searcher" precedent in appbuild. Do NOT
fail Open.

6. **Consumer bridge (dataentry), ENTITIES ONLY.** In NewApp/StartWatching:
`events, cancel := a.store.Subscribe(N); go a.pumpStoreEvents(events)`. Map
EventEntityCreated/Updated/Deleted -> broadcastEntityEvent; IGNORE
EventRelation* (status quo — RR-GNS360). Store cancel for shutdown. Remove the 3
inline entity broadcasts (api_v1.go:592/905/963) ONLY.

**Files to modify / create:**
- pgstore: NEW listener.go (+ origin nonce, channel resolution, reconnect, catch-up);
EDIT entity.go/relation.go/attachment.go (wrap the 5 single-stmt writes in tx +
notify in ALL write paths); EDIT pgstore.go (originID field, Close stops
listener)
  + open.go (start listener, pass DSN/conn-factory); NEW listener_test.go (DB-gated:
cross-process, catch-up, skew, parallel-channel-isolation).
- dataentry: EDIT watcher.go/app.go (store-event pump, entity-only bridge, shutdown
cancel); EDIT api_v1.go (remove ONLY the 3 entity broadcasts).
- appbuild: EDIT appbuild_postgres.go if Open signature changes.
- docs: postgres-backend guide + CLAUDE.md (multi-writer; schema-shared constraint;
LISTEN connection; entity-only live feed; xid8 upgrade note).

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- **NOTIFY payload** is produced by our writers, consumed by our listener — but
the listener MUST be defensive (a DB superuser could `NOTIFY <channel>,
'garbage'`): unparseable payload -> log + trigger catch-up, never panic, never
trust. The catch-up (querying real rows) is the trust boundary; the payload is
only a hint. Delimiter chosen to not collide with validateID-permitted chars.
- **Channel name** derived from `current_schema()` (our own schema, not user
input); a constant prefix + the schema identifier. No injection (quote the
identifier when building the LISTEN statement, or use a parameter where possible
— LISTEN doesn't take params, so the schema name must be a validated identifier;
current_schema() returns a safe identifier).

**Security-Sensitive Operations:**
- Listener holds a long-lived conn; closed on Close (no leak; verify under -race).
- DSN handling unchanged (env-only).

## Test Plan

- [x] Scenarios per AC (see AC1-7)
- [x] Edge cases
- [x] Negative tests
- [x] Integration approach

**Integration approach:** two `pgstore` instances on ONE test schema exercise
real cross-process behavior; schema-scoped channel makes parallel tests safe
(AC7). DB-gated; CI postgres job runs them.

**Edge Cases:**
- Listener conn drops -> reconnect + catch-up recovers; no miss after recovery.
- Local write -> seen once locally (in-process emit), NOT re-emitted by listener
(origin filter).
- Notification for a row already deleted by a later write -> consumer
re-snapshots, finds it gone, handles (delete event / not-found).
- Empty store -> listener idles on timeout; catch-up returns nothing.
- Overlap window: in-window interleave tested; window size documented vs max txn
duration.
- Malformed/garbage NOTIFY -> skip + catch-up, no panic.
- Unicode/special chars in IDs in payload (delimiter safety).
- Parallel tests on different schemas -> no cross-talk (AC7).
- originID changes on restart -> harmless (catch-up idempotent).

**Negative Tests:**
- Garbage NOTIFY -> listener logs, no crash, falls back to catch-up.
- Listener can't connect at Open -> Open succeeds, loud warning, store + local
events still work (degrade, per Approach §5).

## Risk Assessment

- [x] Technical risks assessed
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**
- R1 self-echo double-emit -> per-store originID in payload; listener skips self
(RR-97VOON).
- R2 SSE de-dup regression -> remove ONLY the 3 entity broadcasts; bridge maps
entity ops 1:1; AC4 asserts exactly-once + no-miss (RR-MZOKST).
- R3 overlap-window skew miss under long txn -> size generously; document; xid8
upgrade path.
- R4 connection budget -> standalone LISTEN conn (don't shrink query pool); doc.
- R5 listener/goroutine leak -> Close stops listener first; verify under -race +
goroutine-leak check.
- R6 NOTIFY-not-atomic for single-stmt writes -> wrap in tx (RR-ITQN87).
- R7 channel cross-talk -> schema-scoped channel (RR-CPZGAK); AC7 asserts.
- R8 relation live-reload gap -> conscious out-of-scope decision (RR-GNS360);
documented; no regression vs today.

**Effort:** L.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/postgres-backend.md — multi-writer supported; how the feed works; the
shared-schema constraint; the dedicated LISTEN connection; entity-only live
feed; relations/attachments not in live feed.
- [x] CLAUDE.md — update the single-writer note -> multi-writer via LISTEN/NOTIFY
  + seq catch-up; schema-scoped channel; origin-nonce self-echo pattern;
overlap-window + xid8-upgrade.
- [x] N/A: cli-reference, metamodel.md.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings (all incorporated above):**
- RR-ITQN87 (critical) — NOTIFY can't be in-tx for 5 single-stmt writes -> wrap them in tx.
- RR-CPZGAK (critical) — LISTEN is DB-global -> schema-scoped channel; AC7 added.
- RR-GNS360 (significant) — relations/attachments not in SSE -> entity-only bridge, documented.
- RR-97VOON (significant) — no per-store nonce -> add originID, self-echo filter.
- RR-MZOKST (minor) — remove ONLY the 3 entity broadcasts; keep git/refresh.
