---
id: TKT-92JL8P
type: ticket
title: 'pgstore relation versioning: extend time-machine history to relation props + content'
kind: enhancement
priority: medium
effort: l
status: in-progress
---

## Problem

`entity.Relation` has the same rich content shape as `entity.Entity` — a
`Properties map[string]interface{}` and a `Content` markdown body (see
`internal/entity/entity.go:208`) — but the pgstore versioning system built in
TKT-9INY0Y is **entity-only**. Verified across the whole path:

- `store.VersionInput` / `VersionSnapshot` carry `EntityID` + entity content/props; no relation shape exists.
- The sweep (`sweep.go`) scans `FROM entities` only; relation create/update never produces a version row.
- The synchronous hook fires on `RenameEntity` / `DeleteEntity`, never on `CreateRelation` / `UpdateRelation` / `DeleteRelation`.
- Restore (`history_restore.go:25`) explicitly does NOT restore the relation set.

So editing a relation's properties or body, or adding/removing a relation, is
invisible to history and irreversible via restore. The `relations` table already
has `properties JSONB`, `content TEXT`, `updated_at`, a composite PK `(from_id,
rel_type, to_id)`, and `Create/Update/DeleteRelation` store ops.

## DESIGN — revised after design-review (go-architect + cranky)

The naive "mirror the entity design" is unsafe because relations differ in four
load-bearing ways: no stable id + **mutable composite PK**, destruction via a
**store-level cascade** below the entitymanager, **no field-redaction path**,
and identity **collision** on endpoint rename. The revised design below reflects
the resolved review-responses (RR-181AFY, RR-EZ4I5Q, RR-N5YK81, RR-BZNL0S,
RR-SDDYZO, RR-I3G8A2, RR-S4W5KI, RR-CCITK3, RR-7NYMJK).

### Identity — `rel_record_id` lives ON the relations row (RR-EZ4I5Q)

Relations have no stable id and the composite key mutates on endpoint rename.
Add a surrogate `rel_record_id BIGINT` **column on the `relations` table**,
`DEFAULT nextval('relation_record_seq')`, assigned at `CreateRelation`, carried
**verbatim** through the rename-cascade UPDATEs, dropped with the row on delete.
This dissolves the sweep-vs-sync allocation race and the reused-id-merge /
delete-recreate-dedup class: lineage is read straight off the row,
delete+recreate of the same triple naturally gets a fresh id, and rename-capture
is a plain read. The sweep resolves the live row's `rel_record_id` under the
advisory lock it already holds; it never allocates or reconstructs identity from
the composite key.

### Storage — migration `0005_relation_versions.sql`

```
ALTER TABLE relations ADD COLUMN rel_record_id BIGINT
    NOT NULL DEFAULT nextval('relation_record_seq');   -- surrogate lineage id

relation_versions(
  rel_record_id  BIGINT      NOT NULL,   -- lineage id, copied from the relations row
  vseq           BIGINT      NOT NULL DEFAULT nextval('version_seq'),  -- REUSE version_seq
  from_id, rel_type, to_id   TEXT COLLATE "C" NOT NULL,  -- composite as-of this version
  op             version_op  NOT NULL,   -- create/update/rename/delete
  prev_from, prev_to         TEXT,       -- set on rename cascade (1:1 re-key only)
  properties     JSONB NOT NULL,
  content        TEXT  NOT NULL,
  content_hash   TEXT  NOT NULL,         -- HashRelation: folds in the triple (RR-7NYMJK M1)
  schema_hash    TEXT  NOT NULL REFERENCES schema_versions,  -- REUSE schema_versions
  principal_user, principal_tool, triggered_by  TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (rel_record_id, vseq)
)

CREATE INDEX relations_updated_at_idx ON relations (updated_at);  -- sweep filter (RR-7NYMJK M2)
```

Reuse `version_seq` (NOT `rela_seq`) and the content-addressed `schema_versions`
table unchanged (hash only relation-relevant projection fields). **NO
`from_vseq`/`to_vseq` columns** — see next.

### Endpoint versions — resolved at READ time, not stored (RR-S4W5KI / RR-SDDYZO)

Do NOT store `from_vseq`/`to_vseq`. They cannot be real FKs (entity_versions PK
is composite and `from_id` mutates), would be NULL for most freshly-created
relations (endpoint create is debounced), and would leak a TO-side oracle.
Instead store only the endpoint ids (already the composite) and, at read time,
resolve "endpoint entity version as-of this relation's `vseq`" via the existing
lineage CTE — applying the **reader's own ACL** to each endpoint, so a TO the
reader can't see renders as "endpoint hidden," not as a version reference.

### Capture — hybrid

- **Sweep** (`sweep.go`): a second candidate scan `FROM relations r` LEFT JOIN
LATERAL `relation_versions` on `rel_record_id`, using `relations.updated_at` +
the same debounce / staleness / content-hash dedup. Captures `create`/`update`
under the `version-sweep` principal. Deterministic entities-before-relations
ordering within the one advisory-locked tick (RR-7NYMJK M4).
- **Synchronous delete — via `DeleteResult.DeletedRelations` (RR-181AFY, the
critical one):** the dominant relation-death is `Store.DeleteEntity`'s cascade
bulk `DELETE FROM relations` (entity.go ~L317), BELOW the entitymanager. Make
`DeleteResult.DeletedRelations` (already materialized pre-delete, entity.go
~L307/L352) the **single source** for ALL relation-delete capture — cascade AND
explicit `DeleteRelation` — so there is one path, not two that drift. Each
carries the full pre-delete snapshot + `rel_record_id`, attributed from ctx, in
the delete tx.
- **Synchronous rename (RR-N5YK81):** the cascade UPDATE does NOT bump
`updated_at` (sweep-blind) and has NO `ON CONFLICT`. Keep rename **aborting** on
relation-PK collision (status quo). Capture a `rename` version (with
`prev_from`/`prev_to`, same `rel_record_id`) **post-commit** in the
entitymanager hook by extending `RenameResult` with the affected relations —
keeps attribution at the boundary. A `rename` version is written ONLY for a 1:1
re-key; a colliding rename aborts and writes nothing (a future `merge` op, if
ever wanted, must write a terminal `delete` for the absorbed lineage, never a
silent PK overwrite).
- **Create+delete inside the debounce window (RR-I3G8A2):** the sync `delete`
carries the full pre-delete snapshot even when no `create` was ever swept; the
timeline renders a lone `delete` as "created and deleted within the debounce
window." A re-created triple after delete starts a fresh `rel_record_id`.

### Read / restore / ACL

- **Separate optional interfaces** `RelationHistoryReader` /
`RelationVersionWriter`, type-asserted independently — do NOT fatten entity
`HistoryReader`/`VersionWriter` (consumer-side-interface rule + plimsoll
`max-exported-methods=34` on `pgstore.Store`) (RR-CCITK3 / architect S2).
- **Dual-endpoint gating (RR-SDDYZO):** relation-history read requires the read
verdict on BOTH endpoints (FROM ∧ TO); deleted-relation history uses the global
`history:read`. The "FROM entity owns the history" decision governs UI PLACEMENT
only, never authorization.
- **Field redaction reality (RR-BZNL0S):** relations have NO field-level
redaction anywhere today (`Relation.Inaccessible` is never populated; live GET
emits `rel["meta"]` raw). This ticket does NOT claim a "same serializer path."
It scopes to: relation history exposes exactly what a live relation GET exposes,
pinned by a test — defensible only because dual-endpoint gating (above) covers
the relation's visibility. Building a relation field-redaction path is a
separate follow-up.
- **Restore via Manager (RR-CCITK3):** relation restore calls
`Manager.CreateRelation`/`UpdateRelation` (not a raw store upsert) so the
endpoint-existence check, `ValidateRelation`, and (future) field-write gate all
fire. Restoring a relation whose endpoint entity is gone maps
`ErrEntityNotFound` → **409 dangling-edge**, not 500.

### UI — FROM entity owns the history (placement only)

Relation history surfaces on the FROM-entity detail page — each outgoing
relation gets a history affordance reusing `HistoryView`'s timeline / diff /
restore. The read-time-resolved endpoint versions render with per-endpoint ACL
("TO@v7" or "endpoint hidden").

## Scope notes

- Diff uses the Unix-philosophy split: CLI serializes a relation-version snapshot
(JSON) for piping; frontend renders prop-label + content diffs.
- **Out of scope (explicit follow-ups):** a relation field-level redaction path;
a `merge` op for colliding renames; a full "graph as-of version N" query engine.

## Follow-up origin

Surfaced while completing TKT-9INY0Y — the entity-versioning docs already flag
"relation history is a separate future capability" as a documented boundary.
This ticket is that capability.
