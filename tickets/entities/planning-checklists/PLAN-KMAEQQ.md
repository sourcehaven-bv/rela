---
id: PLAN-KMAEQQ
type: planning-checklist
title: 'Planning: Two-way fsstore↔pgstore sync: hash-based push/pull with manual conflict resolution'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Two-way sync between a local fsstore repo and a remote pgstore
rela-server (FEAT-NJ9FEN). Push primary, pull in scope. Conflicts detected but
resolved **manually** via CLI force flags. No auto-merge, no CRDTs.
**Attachments explicitly OUT of scope** (see design review RR-1IBB49). Full
in/out list lives in TKT-WE01O5.

**Acceptance Criteria:**
1. Laptop fsstore repo pushes local create/update/delete to pgstore server; both ends converge. → integration test: edit local, `rela sync push`, assert server state.
2. Server-side create/update/delete pulls back to local. → integration test: edit via pgstore, `rela sync pull`, assert local files.
3. Concurrent edit to same record → sync halts with clear report; `--force` resolves + re-baselines. → test both push-412 and pull-both-dirty paths.
4. Manifest produced from indexed column + tombstones, not full rescan; reflects deletes. → test delete propagation + EXPLAIN shows index scan.
5. Pushed writes go through entitymanager (ACL/validation/automation-policy/audit). → assert audit record written, validation rejects bad content with 422 (not 412).
6. A push/pull batch applies in a defined order (entities before relations) and a mid-batch failure is recoverable by resuming from the last good cursor (no permanent partial graph). → integration test: inject failure mid-batch, re-run, assert convergence.

## Research

- [x] For larger features: run `/research` — N/A, prior-art survey done inline (below) + lives in FEAT-NJ9FEN
- [x] Searched for existing libraries — sync-engine prior art surveyed (web)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (prior-art rationale captured in FEAT-NJ9FEN body)

**Existing Solutions:** Prior-art survey (Jun 2026): **CouchDB**
`_changes?since` seq cursor + `_revs_diff` + `_local/` checkpoint → blueprint
for cursor/manifest. **PowerSync** uploads PUT/PATCH/DELETE through the
developer's backend write API (validates/authorizes) — exactly our "apply
through entitymanager" rule; confirms it's the normal pattern, not a hack.
**Dolt** cell-level `(pk,column)` three-way merge → the granularity model *if*
we ever add auto-merge (deferred). **Replicache** server-authoritative rebase →
mental model for two-way. **CRDTs rejected** — bypass validation hooks;
ElectricSQL (CRDT inventors) dropped them in their 2024 rebuild.

Codebase prior art / reusable:
- `internal/conflict/` (`resolve.go`) — git-marker per-property resolution; reusable for
presenting a conflict, not for auto-merge.
- pg multi-writer feed (`feed.go`, `listener.go`) — existing `seq > watermark` catch-up with
overlap window; the cursor mechanism mirrors this.
- `fsstore.formatEntity`/`formatRelation` (`internal/store/fsstore/markdown.go:384,480`) —
closest existing entity→canonical-bytes; must be extracted/shared (see
Approach).
- internal `upsertEntity` (`core.go:268-280`) — preserves id, create-then-update; the
shape of the new PUBLIC upsert method we need (RR-L1MY0N), but must keep
ACL/audit/automation framing.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

VERIFIED AGAINST CODE — assumptions checked and design-review findings folded
in:

1. **Canonical hash (assumption #1 FAILED).** No shared serializer. fsstore reflows
the body (goldmark + 80-col wrap) and orders keys by `schema.PropertyOrder`
(`fsstore/markdown.go:384,399`); pgstore stores raw structured columns, never
renders markdown, doesn't load schemas (`pgstore/entity.go:203,506`). Hashing
storage bytes would never match. **Decision:** new shared canonical function
(new `internal/canonical` pkg) turning an `entity.Entity`/`entity.Relation` into
ONE deterministic byte form — schema-independent key order, normalized value
types (watch JSONB↔YAML coercion `pgstore/entity.go:538`), single body rule
applied identically both sides. Both backends hash the reconstructed
`entity.Entity`, NOT stored bytes. `fsstore/echo.go:46 hashContent` is
on-disk-bytes only; not reusable.

2. **Tombstones (assumption #3 FAILED).** All pgstore deletes are HARD deletes
(`entity.go:324`, `relation.go:217`, `attachment.go:85`); `seq > X` catch-up
reads live rows only (`listener.go:232-239`) and `catchUpEvent` only emits
Updated, never Deleted (`listener.go:276-281`). Seq manifest can't discover
deletions. **Decision:** `deletions` tombstone table (`kind, id_a, id_b, id_c,
seq DEFAULT nextval('rela_seq'), deleted_at`) inserted in the SAME tx as each
DELETE; manifest = live `(id,seq>X)` UNION tombstones `(seq>X)`. Soft-delete
rejected (forces `deleted_at IS NULL` on every read/list/search/cascade path).

3. **seq bumped on every surviving-row write** (verified, `entity.go:260,387,401`;
`relation.go:189`); only the rename search_text patch doesn't bump
(`entity.go:393`, benign). **But seq is UNINDEXED** → add B-tree on
`entities(seq)`, `relations(seq)`, `deletions(seq)`.

4. **Cursor stays opaque** (server mints `{seq watermark}` internally; client echoes
verbatim). MVP may return full manifest; later back with indexed seq for
O(changes) delta, no client change. Hash is the `If-Match` token and makes an
over-delivering cursor a safe no-op.

5. **Apply path — NEW public upsert + ordering + automation policy (design review).**
   - **(RR-L1MY0N, CRIT) ID-preserving upsert.** `CreateEntity` rejects an explicit
id for non-manual id_type, and `short` is the default (`manager.go:334-343`,
`core.go:45-53`, `metamodel/types.go:200-205`). There is NO public
upsert-with-id on the EntityManager interface. **Decision:** add a public
`ApplyEntity`/`ApplyRelation` manager method that preserves the supplied id,
does create-or-update, and keeps ACL + audit + validation. Model on internal
`upsertEntity` (`core.go:268-280`) but with full public framing. Both the server
push apply and the local pull apply use it. (Without this, sync cannot create a
record on the peer at all.)
   - **(RR-YHGJHG, CRIT) ordering + atomicity.** `CreateRelation` requires BOTH
endpoints to exist and need their types (`manager.go:667-674,684`); there is NO
cross-call transaction (`entitymanager.go:29-60`). **Decision:** the apply layer
topologically orders a batch — ALL entities (and entity deletes ordered after
relation deletes) before any relation referencing them. Mid-batch failure must
be recoverable: apply is **per-record idempotent** (upsert + hash no-op) and the
cursor/index only advances past records confirmed applied, so a re-run resumes
and converges. Document that there is no global atomic batch — convergence comes
from idempotent replay, not a transaction.
   - **(RR-AZMA7T, SIG) automation suppression.** `CreateEntity`/`UpdateEntity` run
`automation.Process` + cascade (`manager.go:45-63`). Applying a pulled change
could trigger automations that mutate other entities (e.g. status→checklist),
which then look locally-dirty and push back → sync loop / double side-effects.
**Decision:** apply runs in a **suppress-automation/cascade mode** (a context
flag or an apply-mode arg on the new `ApplyEntity` method) — the origin already
ran the automation and its derived changes sync as their own records. Validation
+ ACL + audit still run; only automation/cascade are suppressed on apply. Add a
test asserting no derived writes occur on apply.

6. **(RR-1IBB49, SIG) Attachments scoped OUT.** Attachments are separate
(`AttachmentManager`, `store.go:209-224`), not in `entity.Entity`
(`entity.go:43-50`), and an attach does NOT bump the entity row seq/updated_at
(`attachment.go:60-62`) — so they're invisible to both hash and seq manifest.
**Decision:** explicitly out of scope for this ticket; documented limitation. A
follow-up ticket adds a per-(entityID,property) attachment sync channel with its
own hash/seq + tombstone.

**Files to modify (anticipated):**
- NEW `internal/canonical/` — shared canonical bytes + hash for entity/relation.
- `internal/store/fsstore/markdown.go` — hash via shared canonical.
- `internal/entitymanager/` — NEW public `ApplyEntity`/`ApplyRelation` (id-preserving upsert, ACL/audit kept, automation suppressible).
- `internal/store/pgstore/migrations/000X_sync.sql` — `deletions` table, seq indexes.
- `internal/store/pgstore/{entity,relation}.go` — write tombstone rows on delete.
- `internal/store/pgstore/listener.go` — catch-up/feed aware of tombstones.
- NEW sync HTTP API (data-entry/rela-server) — `PUT /sync/<kind>/<id>` (If-Match), `GET /sync/manifest?cursor=`, content fetch; **authenticated** (see Security).
- `internal/principal/principal.go` — NEW `ToolSync` constant.
- NEW client: sync index (`.rela/sync-state.json`), topo-order diff engine, HTTP client.
- NEW `internal/cli/sync*.go` — `rela sync push|pull [--force <id>]`.

**Dependencies:** existing `entitymanager`, `principal`, `store` events,
`appbuild`, `automation` (to suppress). No new external libs (stdlib
crypto/sha256, net/http).

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Pushed entity/relation content from a remote client → MUST flow through
`entitymanager` validation + ACL (never raw store write). Reject invalid with
- `id`/`kind` path params → **allowlist** validate (known type/prefix, allowed id
charset) and reject path-traversal-shaped ids BEFORE the store.
- Opaque cursor from client → untrusted; parse defensively, parameterized query on
the seq value; malformed cursor degrades to full-manifest, never errors/injects.
- `If-Match` header → exact-match compare only; no parsing into queries.

**Security-Sensitive Operations:** (RR-JDHDJS)
- **Sync endpoint auth is mandatory** — this is a network WRITE endpoint applying
arbitrary content. **Decision needed at impl start:** transport auth = bearer
token / mTLS / reuse data-entry auth — **NOT a bare trusted
`--principal-header`** (an unauthenticated principal header is forgeable →
audit-log spoofing). The principal header may set *attribution* only once the
*request* is authenticated.
- Attribution: add `ToolSync` constant (`principal.go:40-46` has no sync tool;
today a sync write would mis-attribute as `data-entry`). Principal is
per-request (manager reads ctx once/call) → a multi-author batch carries one
principal; decide one-request-per-author vs attribute-to-syncing-user (default:
syncing user).
- DSN/credentials unchanged (server-side `RELA_DATABASE_URL`).
- 412 vs 422 must not leak internal DB state; conflict reports show ids + which side
moved, not raw internals.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** (map to acceptance criteria 1-6)
- Canonical hash: same logical entity via fsstore and via pgstore → identical hash
(linchpin; table-driven over numbers, lists, multiline body, unicode).
- Push happy / 412 stale-base / 422 invalid-content — three distinct assertions.
- Pull create/update/delete (tombstone) propagation; over-delivered unchanged → no-op.
- Conflict both-dirty halt + `--force push`/`--force pull` re-baseline.
- Manifest `seq > X` returns only changed + tombstones; EXPLAIN uses seq index.
- ApplyEntity preserves a `short` id on the peer; ApplyRelation after both endpoints applied.
- Topo-order: a batch with relation listed before its endpoint still applies (reordered).
- Mid-batch failure → re-run resumes from cursor → converges (idempotent replay).
- Automation suppression: applying a status-change pull does NOT auto-create a checklist locally.

**Edge Cases:**
- Delete then recreate same id (tombstone + live row both > cursor).
- Empty repo / first sync (no cursor) → full manifest.
- Missed NOTIFY / cursor gap → catch-up recovers, INCLUDING deletes (the bug we fix).
- Number/type coercion JSONB↔YAML (`normalizeJSONNumbers`) → hash mismatch — explicit test.
- Relation rename re-points relations (seq bumped) — manifest reflects.
- Concurrent writers on server while manifest computed (overlap window).
- Relation whose endpoint was deleted on the peer (apply order: relation deletes before entity deletes).

**Negative Tests:**
- Invalid content pushed → 422, NOT applied, NOT a conflict, no audit for failed write.
- Unauthenticated push → rejected (auth), before any principal/attribution is honored.
- Forged principal header without auth → rejected.
- Malformed/forged cursor → degrades to full manifest, no SQL error.
- Path-traversal id → rejected before store.
- Force on a non-existent id → clear error, no partial state.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- **Canonical-hash divergence (HIGH).** fs≠pg bytes → every push 412s. Mitigation:
single shared fn, exhaustive cross-backend equivalence test, hash reconstructed
`entity.Entity` not storage bytes. Make-or-break.
- **Tombstone correctness (HIGH).** Missed tombstone = silent failed delete on peer.
Mitigation: tombstone in same tx as delete; catch-up scans tombstones; test the
missed-NOTIFY recovery path.
- **New public upsert correctness (HIGH, from RR-L1MY0N).** Must preserve id AND keep
ACL/audit while suppressing automation on apply. Mitigation: build on internal
upsertEntity, full test of the framing.
- **Sync loop / double side-effects (from RR-AZMA7T).** Mitigation: automation
suppressed on apply; test no derived writes.
- **Partial-apply (from RR-YHGJHG).** No batch atomicity. Mitigation: topo-order +
idempotent replay + cursor advances only past confirmed records.
- **Effort.** Canonical serializer + tombstones/indexes + new upsert method + sync
API + auth + client + CLI is genuinely **L**. **Recommend splitting into
sub-tickets**: (a) canonical hash, (b) pgstore tombstones+indexes, (c) public
ApplyEntity/ApplyRelation + automation suppression, (d) sync API + auth, (e)
client + CLI.

**Effort:** l (set on ticket)

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/cli-reference.md — new `rela sync push|pull [--force]` commands
- [x] CLAUDE.md / pgstore section — tombstone table + seq indexes + canonical-hash invariant + the new ApplyEntity write path + ToolSync
- [ ] ~~docs/metamodel.md~~ (N/A: no metamodel changes)
- [ ] ~~docs/data-entry.md~~ (N/A: sync is CLI-level; revisit if a UI surfaces)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** (all addressed in Approach/Security above)
- **RR-YHGJHG** (critical) — relation endpoint-existence + no batch atomicity → topo-order + idempotent-replay recovery (Approach §5, AC #6).
- **RR-L1MY0N** (critical) — no public ID-preserving upsert; explicit id rejected for short/sequential → new public `ApplyEntity`/`ApplyRelation` (Approach §5).
- **RR-1IBB49** (significant) — attachments outside hash/seq → explicitly scoped OUT, follow-up ticket (Approach §6, Scope).
- **RR-JDHDJS** (significant) — sync endpoint auth undecided + no ToolSync → mandatory transport auth, ToolSync constant, per-request principal decision (Security).
- **RR-AZMA7T** (significant) — automations fire on apply → sync loops/double effects → apply in automation-suppressed mode (Approach §5).
