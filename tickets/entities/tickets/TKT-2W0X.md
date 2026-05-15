---
id: TKT-2W0X
type: ticket
title: Lift workspace analysis / attachment / rename-type facades to dedicated packages
kind: refactor
priority: medium
effort: m
status: backlog
---

## Summary

After TKT-0SP1 lands, CLI consumes `*workspace.Workspace`'s CLI-specific
convenience methods through a `cliServices.Analyze` / `.Attach` / `.Rename`
facade that forwards to workspace. Those methods (~700 LOC across
`internal/workspace/analysis.go`, `attachment.go`, `rename_type.go`) are not
workspace-internal concerns — they're CLI-shaped operations that happened to
live on the legacy Workspace god-object.

Move them into dedicated packages so CLI consumes them directly and workspace
shrinks toward deletion.

## In scope

- New `internal/analysis` package owns `AnalyzeAll`, `CheckCardinality`, `FindDuplicates`, `FindGaps`, `FindOrphansWithScope`, `FindOrphanedTempFiles`, `CleanupOrphanedTempFiles`, `RunValidations`, `RunValidationsFiltered`. Constructor takes Store + Meta + Tracer + Validator + FS + Paths (or whatever subset each function needs).
- New `internal/attachment` package owns `AttachFile`, `ListAttachments`. Constructor takes FS + Paths.
- New `internal/renametype` package owns `RenameEntityType`. Constructor takes Store + Meta + FS + Paths.
- CLI's wiring helper constructs each service and exposes it via `cliServices.Analyze` / `.Attach` / `.Rename`.
- Workspace's methods become forwarders (transitional) OR are deleted (preferred — anything else is god-object preservation).

## Out of scope

- Other workspace methods that CLI doesn't call.
- Re-architecting the analysis algorithms themselves.

## Depends on

- TKT-0SP1 — CLI must already consume facades via the bundle (not direct `*Workspace` access).

## Why

The "lift facade methods" half of the original TKT-0SP1 scope. Split out so
reviewers see two reviewable chunks instead of one 1000+ LOC PR. After this
lands, the only remaining `*workspace.Workspace` consumers are dataentry and
scheduler — both already have narrow consumer-side interfaces.

## Risks

- `RunValidationsFiltered` has filter-tag plumbing — make sure the lifted package preserves the existing API surface.
- `AttachFile` interacts with fsstore's attachment directory — verify the lifted package can reach attachment storage without dragging fsstore as a direct dep.
- 700 LOC of moves; mechanical but tests need to follow the implementations.
