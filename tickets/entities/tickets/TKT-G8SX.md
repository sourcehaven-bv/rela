---
id: TKT-G8SX
type: ticket
title: Move project-setup utilities out of workspace into projectsetup
kind: refactor
priority: high
effort: s
status: ready
---

Lift the four free utility functions out of `internal/workspace` into a new
`internal/projectsetup` package:

- `Validate(startDir)` (+ `ValidateWithFS`, `ValidateResult`, `HasErrors`)
- `Migrate(startDir)` (+ `MigrateWithFS`, `MigrateResult`, `MigrateDetection`, `MigrateFile`, `MigrateFileResult`)
- `DetectMigrations(startDir)` (+ `DetectMigrationsWithFS`)
- `Initialize(targetDir)` (+ `InitializeWithFS`, `InitResult`)

These are pure file-IO utilities that don't need the workspace's stateful
services. Moving them frees the rest of `internal/workspace` to be deleted in
TKT-64R3.

Also delete dead code: `workspace.NewAfterInit` (zero callers verified via `grep
-rn NewAfterInit --include="*.go"`).

**Callers to update:**

- `internal/cli/init.go`: `workspace.Initialize` → `projectsetup.Initialize`
- `internal/cli/migrate.go`: `workspace.DetectMigrations`, `workspace.Migrate` → `projectsetup.*`
- `internal/cli/validate.go`: `workspace.Validate` → `projectsetup.Validate` (plus the `ValidateResult` type and `reportDataEntryValidation` parameter)

**`.go-arch-lint.yml` changes:**

- Add `projectsetup: { in: internal/projectsetup }` component
- `projectsetup.mayDependOn`: `dataentryconfig, metamodel, migration, project, storage`
- `cli.mayDependOn`: add `projectsetup` (keep `workspace` for now)
- `workspace.mayDependOn`: drop `dataentryconfig, migration`

**Scope:** ~360 LOC moved + ~100 LOC dead code deleted. No behavior change.

See `.ignored/cli-off-workspace-plan.md` PR 1 for full detail.
