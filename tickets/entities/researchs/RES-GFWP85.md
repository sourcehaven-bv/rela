---
id: RES-GFWP85
type: research
title: 'Face storage alternatives: searchable face-selected content without version-index bloat'
summary: 'Search allows exactly ONE indexed document per entity id (bleve keys docs by e.ID; postgres keeps search_text as a column on the entities PK row), so "index only face-current values" forces a choice between P1 (one privileged indexed face, zero search changes) and P3 (widen the search key to (entity_id, face), large blast radius). Two survey findings constrain both: observers fire from INSIDE store.Store, so face state held on the injected VersionService would never reach the search index; and "fsstore gets versioning from git" is not implemented — internal/git is sync-only, so there is no non-postgres story to inherit.'
status: done
---

## Problem

Follow-up to [[RES-NH3P12]], which recommended storing face state as a ref
table `(entity_id, name) → vseq` into `entity_versions` (option S1). A new
constraint invalidates that recommendation and needs a fresh survey:

> **Full-text search must index only the faces' current values, not every
> version** — otherwise the index bloats with historical content nobody
> searches for.

This reframes the problem. If face-selected content must be searchable, then
it is not Time-Machine data: it is *live-shaped* data with a bounded working set
(`entities × faces`, not `entities × versions`). The question becomes:
**where does face-selected content live such that exactly the current face
targets are indexable?**

## Context

### The decisive constraint: one indexed document per entity id

The whole stack is keyed by **bare entity id, end to end**.

**Postgres** — search is a COLUMN on the `entities` row. There is no search
index table (`internal/store/pgstore/migrations/0001_init.sql:30-52`):

```sql
CREATE TABLE entities (
    id         TEXT        COLLATE "C" PRIMARY KEY,
    type       TEXT        NOT NULL,
    properties JSONB       NOT NULL DEFAULT '{}'::jsonb,
    content    TEXT        NOT NULL DEFAULT '',
    search_text TEXT       NOT NULL DEFAULT '',
    ...
);

CREATE INDEX entities_search_tsv_idx
    ON entities USING GIN (to_tsvector('simple', search_text));
CREATE INDEX entities_search_trgm_idx
    ON entities USING GIN (search_text gin_trgm_ops);
```

`id` is the PRIMARY KEY. **One row = one entity = one searchable blob.**

Incidental: `entities_search_tsv_idx` is created but never queried — no
`to_tsvector`/`to_tsquery`/`@@` in any Go file; the live path is trgm/LIKE only.

**Bleve** — the document key is the bare entity id
(`internal/search/bleveindex/bleveindex.go:110`): `idx.index.Index(e.ID, ...)`.
A second doc under the same key overwrites the first. Fixed candidate ceiling
`req.Size = 10000` (`:258`) — multiplying docs per entity eats that budget.

**LinearSearch** — `map[string]*entity.Entity` keyed by entity id.

**`Backend.Search` returns bare ids** (`internal/search/types.go:33`), and `Hit`
is `{ID, Type, Title}` — **no version field**. "Which version matched" is
structurally unrepresentable without changing both.

### The pipeline assumes the id names a LIVE row

`Service.Search` (`internal/search/index.go:51-97`) gets ids from the backend,
then **re-loads each from the live store** via `reader.GetEntity(ctx, id)`
(`:78`). A load failure is silently skipped as a stale hit (`:79-81`);
`Visible.fieldVisible` does the same (`visible.go:169-174`).

**A face-selected historical version would be dropped by these paths even if
indexed.** P3 is therefore not merely a `Backend` change — it changes the
index→id→live-row pipeline.

### Observers fire from INSIDE the store — the reachability problem

This is the finding that most affects the design, and it cuts against
[[RES-NH3P12]]'s storage conclusion.

`store.EntityObserver` (`internal/store/store.go:784`) has three methods
(`EntityPut`/`EntityDelete`/`EntityRenamed`), carries **no ctx, no principal, no
op kind**, and its errors are discarded by every store (`fsstore.go:342-363`, `_
= o.EntityPut(e)`). It is a derived-state hook, not an event bus.

Critically, it is fired from within the store's own write methods
(`fsstore/entity.go:225,260` call `s.notifyPut`). **So state held OUTSIDE
`store.Store` never reaches an observer.** [[RES-NH3P12]] concluded faces
belong on the injected `VersionService` (correct, per the no-methods-on-Store
rule) — but a pointer move on `VersionService` **would not fire `EntityPut`, so
the search index would never learn the `published` face moved.** That is an
unsolved wiring problem in the earlier recommendation, and it is why P1 (where a
promotion IS a live-row write) is structurally simpler than it first appears.

**The observer seam has exactly one real consumer, ever: search.** Of four
implementations, `pgstore.SearchBackend` is all no-ops
(`pgstore/search.go:39-47`) and one is test-only. The seam has never been proven
to carry a second concern.

Two further traps for any new observer:

- **Backfill is manual.** Observers are NOT invoked for entities already on
disk (`fsstore.go:69-73`); `backfillBleve` (`appbuild_fs.go:73-88`) exists for
exactly this. A face observer needs its own backfill story.
- **pgstore never calls `notifyRenamed`.** Its rename emits
`s.notifyDelete(oldID); s.notifyPut(renamed)` (`pgstore/entity.go:484-485`)
instead of the single callback the contract mandates (`store.go:799-802`).
Harmless *today* only because the pg search backend is a no-op. Any observer
with real behaviour would see a transient delete on rename.

### "fsstore gets versioning from git" is NOT implemented

[[RES-NH3P12]] recorded this as the portability escape hatch for non-postgres
builds. **It is aspirational, not code.**

- `internal/git` is a **sync package**: `Ops` exposes only `GetStatus`,
`Fetch`, `Sync`, `AbortMerge`, `AnalyzeChanges`. Package doc
(`internal/git/git.go:1`): *"provides git operations for the data entry app."*
- The **only** `repo.Log()` call in the tree (`git.go:311`) is inside
`commitsBehind` — counting how far HEAD trails the remote. **No history read, no
blob read, no ref manipulation.**
- **fsstore does not import `internal/git` at all** (verified).
- Commits are authored as a fixed bot signature `rela <rela@local>`
(`git.go:~150`), so git commits cannot attribute a principal even in principle.
- The claim appears only in two build-tag stubs that return nil
(`appbuild/versionsweep_nosweep.go`, `store.go:755-758`).
- Every history consumer nil-checks and reports unsupported
(`cli/history.go:26-31`; `dataentry/history_handler.go:45-52` → HTTP 501).

**Consequence: there is no non-postgres versioning story to inherit.** A face
design that assumes one must build it from scratch (real git plumbing: blob
reads, ref writes, principal-attributed commits).

### fsstore is strictly one file per entity, one state

`entities/<plural>/<id>.md` (`fsstore.go:365-378`), written via temp-file +
rename, **overwriting in place** (`markdown.go:282-290`, `entity.go:235`).
**There is no second content slot on disk for a pointer to point at.**

### No precedent for indexing non-live content

Relations are excluded by contract (`0001_init.sql:9-10`). Attachments have zero
search integration. Version tables carry full snapshots but **no `search_text`
and no GIN index**. Tombstones (`deletions`) model "not a live row" but for the
sync feed, not search.

### Precedent for control-plane state (for the face-move op itself)

`OpPurgeVersion` (`internal/audit/audit.go:71-78`) is the closest analogue:
control-plane state, own storage, own audit constant, emitted directly to the
shared `audit.Audit` sink, explicitly documented as NOT routing through the
Manager (`cli/cli_wiring.go:48-52`).

**But copy it only for audit, not authorization.** Purge buys its bypass by
giving up ACL, observers, and the state machine, compensating with interactive
`--commit` confirmation. A face move is routine and user-facing, so
inheriting that shape would give it *weaker* authorization than an ordinary
property edit — backwards, given [[RES-NH3P12]] makes the face the ACL
subject.

The better authorization template is the **state-machine enforcer**
(`entitymanager/manager.go:606-618`): required, injected, in the fixed write
pipeline, "the unforgettable chokepoint", with per-edge guard permissions and
403/422 sentinels. Its state happens to live in content, but its *enforcement
shape* is the one to copy.

Two constraints on where the op can live: don't add methods to `store.Store`
(`store.go:755-770`), and don't extend `EntityManager`, marked *"Transitional …
Slated for removal"* in favour of narrow consumer-side interfaces
(`entitymanager.go:22-25`).

### Why S1 fails this constraint

S1 put face content in `entity_versions`. That table has no search index,
**no observer fan-out**, no change-feed integration, and no graph among version
rows. Making a subset searchable means adding an index, an observer, and a
change feed *to the Time Machine* — rebuilding live-store capabilities on a
store deliberately built without them. **S1 is withdrawn.**

(Nuance in its favour: `entity_versions` already has PK `(entity_id, vseq)` and
full content, so adding `search_text` + GIN there is a backfillable
derived-column addition. The DDL is cheap; the observer, change-feed, and
pipeline work is what makes it expensive.)

### What still holds from RES-NH3P12

The ACL conclusion — the `@` qualifier syntax and the per-face read verdict —
is unaffected; it concerns the read *gate*, not storage. The blocking
MCP-ungated-reads prerequisite also stands.

## Options

Framed by: **how many independently-searchable states per entity are needed?**

### P1. Privileged face — exactly one face is "the indexed one"

The live `entities` row IS the content selected by one designated face
(declared in the metamodel, e.g. `indexed: true`). Other faces select
immutable snapshots that are retrievable but not searchable.

- **Pros.** **Zero change to search, on any backend.** No `Backend` change, no
`Hit` change, no pipeline change, no new observer. The index is exactly
`entities` — bloat structurally impossible. **Promotion is a live-row write, so
it fires the existing observer fan-out for free** — this sidesteps the
reachability problem entirely, which is a bigger advantage than it first
appears. Backend-agnostic, so it is the only option that works at all on fsstore
given there is no git versioning to build on.
- **Cons.** Inverts the editing model — the live row is the *published* state,
so editing writes "sideways" and promotion swaps. Drafts are second-class (no
search over your own drafts). Only one searchable face per type. **On
fsstore, the un-indexed snapshot side has nowhere to live** — one file per
entity, overwritten in place — so non-postgres builds need a new content store
regardless.
- **Effort.** Small-to-medium on postgres; **larger than it looks on fsstore.**

### P2. Two live rows — face states as ordinary entities

Each (entity, face) pair is a real row in `entities` with distinct ids
(`PAGE-123` / `PAGE-123@draft`).

- **Pros.** Everything works natively — search, traversal, `analyze_*`, ACL,
change feed, observers — with no new machinery. Index grows by exactly the
number of face states: the bounded working set the constraint asks for. **The
only option that solves fsstore and the observer-reachability problem
simultaneously and for free**, because face states ARE store rows.
- **Cons.** Pollutes the id space — every list read, relation, and count must
know `PAGE-123@draft` is not a separate page. Relations become ambiguous. Rename
and id-reuse get harder. High risk of the encoding leaking into user-facing
surfaces.
- **Effort.** Medium in code, large in conceptual blast radius.

### P3. Widen the search key to (entity_id, face)

Face content in a dedicated table; search made face-aware (bleve keys
`"<id>@<face>"`, postgres gets `search_text` per face state).

```sql
CREATE TABLE entity_face_states (
    entity_id   TEXT COLLATE "C" NOT NULL,
    face     TEXT NOT NULL,
    vseq        BIGINT NOT NULL,
    type        TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    properties  JSONB NOT NULL DEFAULT '{}'::jsonb,
    search_text TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (entity_id, face),
    FOREIGN KEY (entity_id, vseq) REFERENCES entity_versions(entity_id, vseq)
);
CREATE INDEX ON entity_face_states USING GIN (search_text gin_trgm_ops);
```

PK `(entity_id, face)` makes row count exactly `entities × faces` —
**bloat bounded by construction**, the constraint as a schema invariant.

- **Pros.** Fully general: N searchable faces, each independently ACL-gated
via the `@` syntax. Keeps `entity_versions` pure Time Machine data; the face
table is a materialized projection of it. `entitySearchText` already works on
any snapshot, so text derivation is free.
- **Cons.** Blast radius beyond `Backend`: `Search`'s `[]string`, `Hit`'s
fields, the id-only observer signatures, **and the live-row re-load with two
stale-hit drop paths**. Needs new observer wiring — and since the table sits
outside `store.Store`, it needs a *new fan-out mechanism*, not just a new
observer. Postgres-only.
- **Effort.** Large.

### P4. Face table + reindex-on-move, single indexed state

Face state in a ref table; one face at a time projected into the live row.

- **Pros.** No search change; the projection makes moves observable via the
existing fan-out.
- **Cons.** Functionally P1 with indirection — still one searchable state. The
projection write makes "move a pointer" a content write, churning `updated_at`,
the change feed, and the version sweep (circular).
- **Effort.** Medium, inheriting P1's limitation without its simplicity.

## Recommendation

**The choice is P1 vs P3, on one question: does more than one face state need
full-text search?** One → P1. Two or more → P3, and nothing cheaper exists,
because one-document-per-id is structural in all three backends.

**P1 is the recommendation**, now on stronger grounds than the search argument
alone: promotion is a live-row write, so it reaches the observer fan-out for
free, whereas any design holding face content outside `store.Store` must
invent a new notification path to keep the index current.

**P2 deserves more credit than a first pass suggests** — it is the only option
that solves fsstore, observer reachability, and search simultaneously with no
new machinery. It is rejected on id-space pollution and relation ambiguity, but
if P1's editing-model inversion proves unacceptable in practice, P2 is the
fallback to revisit rather than P3.

**P4 is dominated by P1. S1 is withdrawn.**

### Tradeoffs accepted under P1

1. **Drafts are not full-text searchable.** Retrievable by id and listable by
face, not discoverable by content search.
2. **The live row means "published."** Editing writes to the un-indexed side;
promotion swaps. Needs clear documentation.
3. **Exactly one searchable face per type.**
4. **Migration cost if P3 is later needed** — not additive: it changes what the
live row means AND widens the search key.
5. **fsstore needs a content store for the un-indexed side.** One file per
entity, overwritten in place, and no git versioning exists to lean on. This is
the largest under-appreciated cost, and it applies to every option.

### Open questions

**1. Is the feature postgres-only?** Given the git-versioning story is
unimplemented and fsstore has one content slot per entity, the honest v1 scope
may be postgres-only — same posture as history today. Decide deliberately rather
than discovering it.

**2. What does the change feed / SSE emit on a pointer move (P1)?** Promotion
rewrites the live row, emitting an ordinary entity event, and makes the version
sweep capture a version whose content equals the promoted snapshot. Either
harmless (content-hash dedup should match) or a duplicate-version bug, depending
on whether the comparison lands in the same lifecycle window. Verify against
`sweep.go`'s two-LATERAL dedup.

**3. Where does the face-move op live?** Not `store.Store` (rule), not
`EntityManager` (slated for removal). Copy `OpPurgeVersion`'s *audit* shape (own
constant, direct sink) but the state machine's *enforcement* shape (required
injected enforcer in the fixed pipeline) — purge's Manager bypass would leave a
routine user op with weaker authorization than a property edit.

**4. Fix pgstore's missing `notifyRenamed` first.** Independent of the option
chosen, any observer with real behaviour on the pg path will hit the delete+put
decomposition. Currently masked by the pg search backend being a no-op.

**5. Should the unused `entities_search_tsv_idx` be dropped?** Incidental — a
maintained GIN index with no query path is pure write amplification. Its own
ticket either way.
