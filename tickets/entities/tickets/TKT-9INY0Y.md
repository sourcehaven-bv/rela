---
id: TKT-9INY0Y
type: ticket
title: 'pgstore content versioning: time-machine history + diff with principal attribution'
kind: enhancement
priority: medium
effort: l
status: in-progress
---

## Problem

fsstore deployments get content versioning for free via the underlying VCS
(git): every write is a commit with an author, a timestamp, and a full-content
diff, and a user can inspect or diff any prior revision. **pgstore has no
equivalent.** `UpdateEntity` / `RenameEntity` do an in-place SQL `UPDATE`
(`internal/store/pgstore/entity.go`), so prior content and properties are
overwritten and lost.

The audit log (`internal/audit`) records *that* a write happened, with
`Principal{User, Tool}` attribution and a summary, but its
`Subject`/`Before`/`After` capture only the subject **identity**, not the prior
**content** (`internal/audit/audit.go:60`). So in a pg-backed deployment there
is no way to reconstruct or diff an earlier version of an entity.

## Goal

Automatically version entity content in the pgstore backend, **time-machine
style**, so a user can:

- **list** prior versions of an entity,
- **view** any past version, and
- **diff** two versions,

with each version carrying **full attribution**: the `Principal` that made the
edit, the timestamp, and the op / triggered-by (automation / schedule / cascade
/ sync).

This closes the fsstore-vs-pgstore capability gap for the `store-backends`
concept and extends the forensic story the `audit-log` concept started — from
"what changed, and by whom" to "what did it look like *before*, and show me the
diff."

## Decisions so far (from scoping discussion, 2026-07-06)

- **Attribution lives at the `entitymanager` boundary**, where audit already
runs and the `Principal` is known — *not* in the raw store. The store never
learns the Principal (consistent with the current audit design). The atomicity
question (two writes vs. a store API that accepts version metadata) is a
planning/research concern.
- **Separate `entity_versions` table**, not system-versioned columns on the
live `entities` table. History is write-heavy / read-rarely and never searched;
keeping it out of the hot table keeps the live PK + GIN indexes (tsvector/trgm)
lean. Mirrors the tombstone precedent.
- **Scope: entities first.** Relations may be *embedded* into the entity's
versioned snapshot (content-addressed by hash for dedup) rather than versioned
as first-class rows — but relation *ownership* (a relation is shared between two
entities; whose history does an edit land in?) is an open design question.
Attachments (BYTEA) history is out of scope for v1.
- **Surfaces: CLI + data-entry UI** (version timeline + diff view). MCP tool
deferred.
- **Schema-drift handling is a core research question** → see the linked
research. The idea: snapshot the relevant metamodel content-addressed by a hash
into a `schema_versions(hash, snapshot, captured_at)` table so a historical
version renders/diffs against the schema it was created under, not today's.
Open: should *live* entities also carry the schema_hash (unlocks "which entities
predate migration X" + faithful rendering everywhere) or only historical version
rows?

## Relevant existing structure

- pgstore already maintains a global monotonic `seq` (from `rela_seq`), bumped on every mutation — a natural version cursor (`migrations/0001_init.sql`).
- The audit log already threads `Principal` through every write at the `entitymanager` boundary — the same attribution a version record needs, and the chosen home for versioning.
- Deletion **tombstones** already exist for the fs↔pg sync manifest (`internal/store/pgstore/tombstone.go`, `FEAT-NJ9FEN`) — prior art for "durable record of a removed row" and the separate-table precedent.
- Metamodel evolves via `internal/migration` — the reason schema-drift matters.

## Out of scope / distinct work

- **Point-in-time ACL evaluation** is the separate epic `IDEA-CQMKMD` (snapshot-versioned ACL). This ticket is about *content* history + diff, not read-verdict-as-of-snapshot — though both lean on the same `seq` marker.
- fsstore/memstore versioning is *not* required — fsstore already has git; this ticket is pgstore-specific.

## Open questions for the research / planning phase

- **Storage shape:** full snapshot per version (bias) vs. delta chains. Full snapshots sidestep schema-evolution reconstruction hazards; retention (cap by age/count) is the pressure valve.
- **Schema-drift:** the schema-hash design above, and how far it reaches (versions only vs. live entities too).
- **Relations:** embed-in-entity-snapshot vs. first-class versioning vs. defer; resolve the shared-ownership asymmetry.
- **Version triggers:** create + update + rename + delete all produce versions? (Needed for a true time machine — incl. pre-delete state for inspect/undelete.) Rename keys history on `entity_id` which itself changes — pairs with tombstone logic.
- **Diff semantics:** line/prose diff for markdown `content`; structured field-level diff for JSONB properties, schema-aware (flag fields present in only one side due to schema change).
- **Retention:** unbounded vs. capped (mirror audit-log 12-month thinking, `BUG-6PYB6G`).
