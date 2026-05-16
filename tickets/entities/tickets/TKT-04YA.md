---
id: TKT-04YA
type: ticket
title: Lift workspace.RenameEntityType to internal/renametype
kind: refactor
priority: medium
effort: s
status: backlog
---

## Summary

Move `Workspace.RenameEntityType` (~130 LOC in
`internal/workspace/rename_type.go`) into a new `internal/renametype` package.
Second of three sequential lifts (after TKT-2W0X).

## In scope

- New `internal/renametype` package with `Service` exposing `Rename(oldType, newType, newPlural) (int, error)`.
- Helpers (`rewriteEntityTypeInDir`, `rewriteEntityTypeInFile`, `replaceYAMLType`) move with implementation.
- `cliAnalyze` interface: `RenameEntityType` signature switches to consume the new package's Service.
- `internal/workspace/rename_type.go` DELETED.
- Tests follow.

## Out of scope

- Attachment / analysis facades — separate tickets.

## Depends on

- TKT-2W0X (attachment lift) — same wiring-helper pattern. Stack on its branch.

## Why

Mid-size facade. Same pattern as TKT-2W0X. Worth doing before the large analysis
lift so the analysis PR is the only one whose review is meaningfully sized.
