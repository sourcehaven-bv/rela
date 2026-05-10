---
id: TKT-0SP1
type: ticket
title: Migrate CLI commands to scoped consumer-side interfaces (off Workspace)
kind: refactor
priority: medium
effort: m
status: ready
---

## Summary

`internal/cli/root.go` holds package-level globals `ws`, `projectCtx`, `meta`
populated via `workspace.Discover` in `PersistentPreRunE`. Subcommands reach
into `ws` for whatever they need: Store, Meta, Tracer, EntityManager, Paths,
Config, plus CLI-specific utilities like `RunValidations`, `FindOrphans`,
`FindGaps`, `FindDuplicates`, `RenameEntityType`, `Templater`, `AttachFile`,
`ListAttachments`.

Replace package globals with explicit, scoped consumer-side interfaces. Each
subcommand declares the minimum surface it needs.

## In scope

- Define a small number of scoped CLI service interfaces in `internal/cli` based on actual usage clusters (rough sketch — finalize during planning):
  - `analyzeServices` — Store, Meta, Tracer, FindOrphans, FindGaps, FindDuplicates (used by `find`, `gc`, etc.)
  - `validationServices` — Store, Meta, RunValidations (used by `validate`)
  - `schemaServices` — Store, Meta, RenameEntityType (used by `schema`, `rename`)
  - `writeServices` — EntityManager, Meta (used by create / update / delete commands)
  - `templateServices` — Templater, Paths
- A small wiring helper builds focused services once at startup and supplies the appropriate interface to each subcommand's `RunE`.
- Drop the `ws`, `projectCtx`, `meta` package globals in `cli/root.go`.

## Out of scope

- Reorganizing the CLI command structure itself.
- Test refactoring beyond what's needed to unblock the migration.

## Depends on

- `entitymanager.Manager` real implementation (separate ticket).
- automation.Runner extraction (separate ticket).
- Possibly "Extract analysis facade" if `FindOrphans` / `FindGaps` / `FindDuplicates` are best lifted into their own service rather than staying as Workspace methods. Decide during planning.

## Why

CLI is the largest cluster of Workspace consumers — ~18 commands. Migrating them
is the bulk of the dependency reduction work. The package globals in `root.go`
are also the single biggest "god-object via the side door" pattern in the
codebase.

## Risks

- The scoped interfaces have to align with actual usage; a survey is needed during planning to avoid creating clusters that don't match real boundaries.
- Some Workspace methods (`RunValidations`, `FindOrphans`, etc.) are CLI-only conveniences. They may need to lift into their own service packages (`internal/analysis`, etc.) rather than staying on Workspace; that's a separable decision.
- Largest blast radius of the migration tickets. May warrant splitting per command-cluster if the PR gets too big.
