---
paths:
  - "internal/datamigration/**"
  - "internal/perfseed/**"
  - "internal/fsimport/**"
  - "internal/cli/migrate*.go"
  - "internal/cli/dev.go"
  - "internal/appbuild/projectdata*.go"
  - "internal/cli/db*.go"
---

# Data migration, perf seeding and markdown import

- **Data migration** (TKT-0C57FS, TKT-XCJ0Y2, `internal/datamigration`,
  `docs/data-migration.md`). Two separate questions, deliberately not
  conflated:

  _Which migrations have run_ is answered by NAME, from a per-store
  `datamigration.StateStore`. A migration file is therefore NOT an edge in a
  hash graph, and a **data-only migration** (backfill, de-dup, correcting an
  old bug's values) is an ordinary file whose two projections match. Files are
  `<14-digit timestamp>-<lowercase-slug>.yaml`, validated through
  `MigrationName` on both the directory listing and the applied list — the two
  are compared by equality, so a case-folding filesystem would otherwise make
  one file look like two entries. Don't reintroduce sequential numbering: it
  collides silently across concurrent branches (BUG-TY2XQC is that defect in
  rela's own pg ladder).

  _What shape the data conforms to_ is the `ShapeProjection` the record also
  stores, which the gate classifies against and `rela migrate gen` diffs.
  **Two schema hashes coexist on purpose**: `RenderProjection` (version
  rendering, `schema_versions` dedup — stability load-bearing, do not extend)
  vs `ShapeProjection`.

  **The projections stay embedded in every migration file.** They are what
  step `Validate(from, to)` checks targets against (a rename step is
  well-formed only where the old property exists in the FROM shape, which the
  live schema no longer has once a later migration ran) and what
  `validateDeltasResolved` recomputes to refuse a file that spans a
  needs-migration change its steps don't answer. Computing either against the
  live schema instead collapses the whole chain into one aggregate delta and
  reopens BUG-TMGWIN. Removing them is not an optimization.

  **The state store is backend-selected**, like `comments.Store`: a COMMITTED
  `migrations/applied.json` on fs (the gitignored `.rela/` would be absent from
  a clone), `migration_state` in the tenant's schema on pg (so tenants at
  different points migrate independently — a documented guarantee), the same
  table in `rela.db` on sqlite (a shipped file must carry it). New backends
  pass `migstatetest.RunAll`.

  **`Gate.Evaluate` classifies; `Gate.Persist` writes, and only the CLI and a
  Configure save call it.** A server writing a git-tracked file at boot would
  dirty a working tree and need a writable project dir; it also removes the
  concurrent-start race outright. A server may serve with an unrecorded
  ADDITIVE change — harmless by construction. With no record AND migrations
  present the gate refuses (`StatusUnbaselined`) rather than baselining over
  files that may still need to run; `rela migrate baseline` is the explicit
  override.

  The Configure save (TKT-F5NGMG, DEC-325POW) is the one server-side writer.
  It is the CLI's flow run from the browser: it adopts the current shape
  (`MigrationState.Adopt`), writes the generated migration file next to the
  config, runs it, and persists — only on `rela-server --config-editing`, only
  for a `config:edit` holder, only on the fs build, and audited as
  `config-edit` plus the runner's `data-migration` records. Do not add a
  second server-side writer, and never persist at boot or on a preview.

  Migration/GC writes are the third sanctioned raw-store exception (after
  `db migrate` and `history-purge`): operator-shell trust (or, for a
  Configure save, the `config:edit` grant, which is admin-equivalent), no
  ACL, explicit audit records (`data-migration`/`data-gc`), `store.WithAttribution`, and
  synchronous pre-delete version capture on pg (the sweep cannot reconstruct
  deleted rows). **Steps must stay idempotent — with the applied list as the
  only double-apply guard, re-run IS the crash recovery.** The Lua step is a
  pure transform (patch in, patch out, engine applies); never hand it a write
  handle.

- **Perf seeding** (TKT-1U8XYN, `internal/perfseed`, `rela dev seed`) is the
  fourth raw-store exception, under the same terms: operator shell, attributed
  (`perf-seed` tool), one `perf-seed` audit record, and it refuses a non-empty
  store. Because nothing above the store runs, the generator keeps the
  invariants the store cannot: ids minted by construction and validated, the
  single `unique:` property unique by construction, every edge endpoint emitted
  by the same generator. Do not route it through entitymanager to "fix" that —
  20k automations per seed is the cost it exists to avoid.

- **Markdown import into SQLite** (TKT-LWOCW9, TKT-YNKKRQ, `internal/fsimport`)
  is the fifth, on the same terms: operator shell, attributed (`fs-import`
  tool), one `fs-import` audit record. Two commands share ONE engine,
  `fsimport.Copy`: `rela db load --data` (`appbuild.ImportMarkdownData`, also
  the desktop app) fills the project's own database, and `rela-sqlite db
  import-fs` builds a new project directory. Do not grow a second copy loop;
  both get the same refusals, date normalization and report. The engine
  copies what an fsstore over the source READ, then walks the source files
  and accounts for every one (copied, listed with a reason, or an error),
  because fsstore skips what it cannot read without a word. The source is
  opened through `storage.ReadOnlyFS` with the fsstore index cache off, so the
  import cannot write to it. The target judges: any row the database refuses
  fails the whole run. `db load --data` runs as ONE `store.Tx`, and the state,
  comment and migration stores write on that transaction's connection
  (`sqlitestore.TxConn`), so a failed import leaves nothing behind and can
  simply be re-run; it refuses a database holding entities unless `--force`,
  and keeps a setting or migration record the database already has.
  `import-fs` stages beside the target and renames only after every write
  landed. A new sqlite table must be listed in
  `TestDBImportFS_SQLiteTablesAreAccounted`, which forces the decision whether
  the import copies it. `ExportMarkdownData` (`rela db dump --data`) is the
  read-side counterpart and writes through a plain fsstore.
