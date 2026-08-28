// Package datamigration migrates STORED CONTENT when the schema's data shape
// changes (TKT-0C57FS). It is distinct from the two other things called
// "migration" in this codebase: internal/migration rewrites operator CONFIG
// files (schema.yaml/data-entry.yaml/acl.yaml syntax upgrades), and
// pgstore.Migrate applies SQL DDL. Both run before anything here does.
//
// # Model
//
// The schema version of a store's content is the hash of
// metamodel.ShapeProjection — a semantic projection, so cosmetic schema edits
// never demand a migration. A per-store marker in state.KV records the shape
// (hash + full projection) the data currently conforms to.
//
// When the live schema's shape differs from the marker, the
// [metamodel.CompareShapes] classifier decides what happens:
//
//   - additive deltas: the Gate adopts the new shape silently
//   - drift deltas (deletions, new required properties, delete+add pairs):
//     the Gate adopts, logs a notice per delta, and records orphaned schema
//     names in the drift ledger for the GC engine
//   - needs-migration deltas: the Gate refuses to adopt and points the
//     operator at `rela migrate gen` / `rela migrate data`
//
// Migrations are operator-authored YAML files in the project's `migrations/`
// directory, keyed from→to shape hash with both projections embedded (the
// file is self-contained: the resolver and plan-time validation never need a
// historical schema.yaml). Steps are declarative (rename_property,
// map_values, convert, set_default, recompute_computed, drop_*) with a Lua escape hatch that is
// a PURE TRANSFORM: the script returns a patch, the runner applies it — Lua
// never holds a write handle, so no entitymanager validation, automations,
// or state machines run mid-migration.
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
