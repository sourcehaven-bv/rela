// Package datamigration migrates STORED CONTENT when the schema's data shape
// changes (TKT-0C57FS). It is distinct from the two other things called
// "migration" in this codebase: internal/migration rewrites operator CONFIG
// files (schema.yaml/data-entry.yaml/acl.yaml syntax upgrades), and
// pgstore.Migrate applies SQL DDL. Both run before anything here does.
//
// # Model
//
// Two questions run this package, and keeping them apart is the whole design
// (TKT-XCJ0Y2):
//
// WHICH MIGRATIONS HAVE RUN is answered by NAME, from the per-store [State] a
// [StateStore] holds. That alone decides what [Resolve] plans, which is what
// makes a DATA-ONLY migration expressible: a backfill or a correction of an
// old bug's values has no schema change at all, and under a model that keyed
// migrations to shape edges it could not be represented.
//
// WHAT SHAPE THE DATA CONFORMS TO is the metamodel.ShapeProjection the same
// record stores — a semantic projection, so cosmetic schema edits never demand
// a migration. Its hash is a cheap equality check, not an identity a migration
// is addressed by.
//
// When the live schema's shape differs from the recorded one, the
// [metamodel.CompareShapes] classifier decides what happens:
//
//   - additive deltas: the Gate adopts the new shape silently
//   - drift deltas (deletions, new required properties, delete+add pairs):
//     the Gate adopts, logs a notice per delta, and records orphaned schema
//     names in the drift ledger for the GC engine
//   - needs-migration deltas: the Gate refuses to adopt and points the
//     operator at `rela migrate gen` / `rela migrate data`
//
// [Gate.Evaluate] only CLASSIFIES; [Gate.Persist] records, and only the CLI
// calls it — a server must not write the record, which on the filesystem tier
// is a git-tracked file.
//
// Migrations are operator-authored YAML files in the project's `migrations/`
// directory, named `<14-digit timestamp>-<lowercase-slug>.yaml` and run in
// name order. Both schema projections stay EMBEDDED in each file: step
// validation checks targets against the shapes that file spans, and
// validateDeltasResolved recomputes the file's own edge to refuse a file whose
// steps do not answer the change it crosses. Neither can be recomputed from
// the live schema once a later migration has run. Steps are declarative
// (rename_property, map_values, convert, set_default, recompute_computed,
// drop_*) with a Lua escape hatch that is a PURE TRANSFORM: the script returns
// a patch, the runner applies it — Lua never holds a write handle, so no
// entitymanager validation, automations, or state machines run mid-migration.
//
// Because the applied list is the ONLY double-apply guard, every step must be
// idempotent: re-running after a crash is the documented recovery path.
//
// # Trust boundary
//
// Execution writes raw to store.Store, deliberately bypassing entitymanager
// (the migration's input is by definition invalid under the new schema).
// The trust boundary is the operator shell — the same sanctioned exception
// as `rela db migrate` and `rela history-purge`: no ACL, attribution via a
// real system principal (store.WithAttribution), one explicit audit record
// per run, and synchronous version capture for destructive/rename steps via
// the optional store.VersionService (pg builds only).
//
// Nothing here is reachable from a served surface: the CLI commands and the
// startup/reload Gate are the only entry points.
package datamigration
