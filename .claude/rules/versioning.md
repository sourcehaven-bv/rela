---
paths:
  - "internal/store/*.go"
  - "internal/store/pgstore/**"
  - "internal/store/sqlitestore/**"
  - "internal/entitymanager/**"
  - "internal/dataentry/history_*.go"
  - "internal/dataentry/historyworld.go"
  - "internal/dataentry/relation_history_handler.go"
  - "internal/cli/history*.go"
  - "internal/cli/relation_history.go"
---

# Content versioning

- **Content versioning** (TKT-9INY0Y; pgstore below, and sqlitestore since
  TKT-4NU9ZD — the two are held to one contract by `storetest.RunVersionTests`,
  declared via `Capabilities{Versioning}`). Two tables (`entity_versions` = one
  full snapshot per version; `schema_versions` = content-addressed render-schema
  projection, deduped) plus a dedicated `version_seq` sequence. **Use
  `version_seq`, never `rela_seq`** — `rela_seq` feeds the change-feed watermark
  (`primeWatermark`/`catchUp` scan entities/relations/deletions), and burning it
  on version rows that don't land in those tables would erode the overlap budget
  and drop real events. Capture is **hybrid**: rename+delete are captured
  synchronously at the entitymanager boundary (they carry old→new id /
  pre-delete state the sweep can't reconstruct); create/update are captured by a
  debounced reconciliation **sweep** goroutine (`sweep.go`, started/stopped like
  the listener in the change feed, see `storage.md`). The sweep's candidate query
  must not keep selecting rows its Go dedup skips: such a row is selected again
  every tick, and a batch of those starves every row behind them (BUG-1DWMYO). It therefore selects only rows whose stored
  `content_hash` is NULL, never by stored columns or timestamps. A non-NULL hash
  means the current lifecycle's latest version has that hash: the sweep writes
  it back after capturing or skipping a row (unless the row or its latest
  version changed since the read), and triggers clear it when a hashed
  column changes, a version is written with another hash or as a delete, a
  version is purged, or a row is inserted (pgstore migrations 0020 and 0021,
  sqlitedb v14 and v15). A partial index on the NULL rows keeps a tick
  proportional to what changed. Any new path that writes or deletes version
  rows is covered by those triggers; a new hashed column must join the update
  trigger's column list. The sweep runs its **entire tick on ONE acquired pool
  connection** under `pg_try_advisory_lock` — the lock is session-scoped, so
  issuing the inserts via the pool (other sessions) would silently void the
  single-writer guarantee. Attribution comes from ctx only, via exactly two
  boundary-populated inputs — the store never learns the Principal by another
  route: sync captures carry it inside `store.VersionInput`, and create/update
  writes carry a `store.Attribution` on ctx (`store.WithAttribution`, set ONLY
  at the entitymanager boundary and ONLY for a real principal — never translate
  a zero/unknown principal, RR-U964M0) which pgstore stamps into
  `entities/relations.last_edited_by_user/_tool` (TKT-ZIRMGM). The sweep copies
  those columns onto swept versions; NULL columns (legacy rows, unattributed
  writes) fall back to the `version-sweep` system principal — never a guessed or
  literal-"unknown" identity. Author-boundary segmentation (flush-on-author-
  change) is TKT-0IGI4V, not built: two authors in one debounce window merge
  into one version attributed to the last of them. Lineage across a
  rename/id-reuse is fenced by `[lo,hi)` vseq ranges in a recursive-CTE walk (an
  unbounded `entity_id = ANY(...)` read would merge two entities' histories —
  see the version.go doc). `HistoryReader`/`VersionWriter` are optional store
  capabilities (type-asserted like `store.Formatter`), NOT part of
  `store.Store`.
- **Relation versioning** (TKT-92JL8P; both database backends) extends the above
  to relations, which carry their own props + body. A `relation_versions` table
  reuses `version_seq` + `schema_versions`; identity is a surrogate
  `rel_record_id` **column ON the `relations` row** (`DEFAULT nextval(...)`,
  carried through writes) — NOT reconstructed per sweep-tick, which would race
  the sync path and merge/fork lineages. Delete+recreate of the same
  `(from,type,to)` mints a fresh id (histories don't merge). Capture:
  create/update via the sweep's second `FROM relations` scan
  (entities-then-relations, same tick/lock); **delete synchronously via
  `DeleteResult.DeletedRelations`** — the single path for BOTH explicit
  `DeleteRelation` and entity **cascade** delete (the store bulk-deletes
  relations below the entitymanager, so cascade edges would otherwise lose
  history). Rename **stitches** (not forks): the entitymanager captures a
  `rename` version per incident relation on the new triple carrying
  `prev_from`/`prev_to`, and `relationLineageIDs` walks those links so history
  is continuous. Since #1127 the store renames **atomically** (bulk in-place
  `UPDATE relations SET from_id=...`), so a relation KEEPS its `rel_record_id`
  across the rename — the lineage is already continuous on one id and the
  `rename` version merely appends a marker (the `prev_from`/`prev_to` stitch walk
  finds no fork; it stays as belt-and-braces for any future non-atomic path).
  Rename capture is **sync-only best-effort**: the atomic re-key does NOT bump
  `relations.updated_at` (TKT-9TQ6I). If the synchronous hook misses a rename,
  the sweep records the new endpoints as an ordinary `update` once the row is
  settled or stale, so only the rename marker is lost, never lineage continuity. Read/restore is gated on **both** endpoints
  (FROM ∧ TO) — the FROM
  entity only _owns_ the UI placement, it is not the auth boundary (a TO-side
  oracle otherwise). Relation meta fields are redacted by a role's
  `relations:` `visible:` grants (`RelationFieldVerdicts`, applied in
  `internal/dataentry`), never by a client ceiling; relation history exposes
  exactly what a live relation GET does. `RelationHistoryReader`/
  `RelationVersionWriter` are SEPARATE optional capabilities, type-asserted
  independently of the entity ones.
- **Version purge** (TKT-BW6UUL; both database backends) is the audited, irreversible
  exception to append-only history — hard-deletes version rows for compliance
  redaction. `VersionPurger`/`RelationVersionPurger` are SEPARATE optional
  capabilities (`purge.go`), one `PurgeVersions`/`PurgeRelationVersions` method
  each. Load-bearing guardrails (design-review, do not relax): the whole op runs
  under **`sweepAdvisoryLockKey`** (mutually exclusive with a sweep tick — a purge
  racing a capture-insert loses the erasure); it **REFUSES while a live row still
  holds the content** unless `--force-live` (else the sweep re-captures it within
  one interval — a `VersionOpPurge` no-content tombstone whose content_hash = the
  live hash suppresses that re-capture: the purge clears the row's stored hash,
  and the sweep then finds the live content equal to the tombstone's); it
  **REFUSES a rename row** (purging one orphans/forks the lineage walk — v1 is
  non-rename-only); `--all` purges the **fenced lineage** (`lineageCTE` /
  `relationLineageIDs`), never `WHERE id=$1` (id-reuse would destroy unrelated
  history). CLI-only (`history-purge`/`relation-history-purge`), dry-run by
  default; trust boundary is operator shell (no ACL check — like `db migrate`),
  audited via the `audit.Audit` sink (`OpPurgeVersion`, `svc.Audit()`), never
  echoing purged content. `schema_versions` is projection-only + FK-shared, so
  purge never deletes it. Purge is necessary-not-sufficient for erasure (live
  row / PITR backups survive) — see the postgres-backend guide.
