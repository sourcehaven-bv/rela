---
id: TKT-FNARO6
type: ticket
title: Rename metamodel.yaml to schema.yaml with backward-compatible dual-name discovery
kind: refactor
priority: medium
effort: m
status: review
---

Consolidate the schema file name on `schema.yaml`, keeping `metamodel.yaml`
readable indefinitely behind a one-shot deprecation warning. Discovery becomes
dual-name in `internal/project.Discover()`; the resolved path is stored on
`Context` so in-place writers rewrite whichever name they found.

## Motivation

The user-facing surface has already drifted to "schema" — this ticket makes the
filename catch up rather than introducing a new opinion:

- CLI command is `rela schema` (`internal/cli/kong.go:90`), whose help string reads `"View the metamodel schema."` — glossing "metamodel" because the word is not self-explanatory alone.
- REST API is `/api/v1/_schema` (`internal/dataentry/api_v1.go:84`), not `_metamodel`.
- Only the filename, the Go package, and the MCP tool `get_metamodel` still say metamodel.

Secondary argument: the file's top-level keys are `version, namespace, types,
entities, relations, validations, automations`. `automations` is behaviour
(triggers and rules), not a description of model shape, so "metamodel" is
arguably no longer accurate for what the file holds.

## Scope

- **`internal/project/context.go`** — dual-name discovery. Add `SchemaFile = "schema.yaml"` and `LegacyMetamodelFile = "metamodel.yaml"`. `Discover()` (line 57 is currently the *only* place that stats for the file) tries `schema.yaml` first, falls back to `metamodel.yaml`. Rename the `MetamodelPath` field to `SchemaPath`.
- **Both files present** — prefer `schema.yaml`, warn. Decision: not an error. If both exist the user is mid-migration and `schema.yaml` is the file they just wrote, so preferring it does the right thing; erroring would block a user whose intent is unambiguous.
- **Deprecation warning shown once** — emitted from the CLI entry point, driven by a `Context` field. Explicitly *not* from `Discover()`, which is called per-command and per-request in `internal/mcp/server.go` and the data-entry paths; warning there would spam logs.
- **`internal/migration`** — add the file rename as a migration step so `rela migrate` performs it. The deprecation warning should name that command.
- **`internal/cli/init.go` + `internal/projectsetup/init.go`** — new projects create `schema.yaml`.
- **MCP** — rename tool `get_metamodel` → `get_schema`, keeping `get_metamodel` as an alias, since MCP clients have the name cached in their configs.
- **Docs and in-repo project files** — `docs/`, `CLAUDE.md`, `tickets/`, `docs-project/`, `examples/`, `prototypes/`.

## Care points (verified during investigation)

- **In-place writers must write back to the name they found.** `internal/renametype/renametype.go:81` calls `metamodel.RenameEntityType(paths.MetamodelPath, ...)` and `internal/projectsetup/migrate.go:145` writes to `ctx.MetamodelPath`. Both are correct for free *if* the resolved path is stored on `Context` rather than recomputed from the new constant. This is why the design stores a resolved path, not a boolean flag.
- **A project can be discovered with neither file present.** `context.go:63-66` already treats a `.rela` directory as an alternative project marker. The "which name did we find" field needs a sensible empty state, and the missing-file error should mention `schema.yaml` while noting the legacy name is still accepted.
- **The blast radius is small despite 905 files mentioning "metamodel".** Only 16 non-test usages of the `MetamodelPath`/`MetamodelFile` constants; every other consumer reads the already-resolved `ctx.MetamodelPath`.
- **The `namespace:` key is inert.** Grep confirms no RDF/ontology exporter consumes it, so the filename does not leak into exported artifacts. No extra compatibility surface there.

## Out of scope

- **Renaming the Go package `internal/metamodel`.** Internal precision is fine, the churn is large, and it can follow as a separate ticket.
- **Moving `automations:` out to its own file.** Discussed as a possibly cleaner fix — it would make "schema" precisely accurate — but deliberately deferred.

## Deprecation window

Support both names for longer than the usual two releases. The shim is roughly
15 lines and one field, so removing it early buys nothing and breaks users who
skipped a version. Deprecate now; keep reading `metamodel.yaml` until a major
version bump.
