---
id: TKT-2W0X
type: ticket
title: Lift workspace.AttachFile / ListAttachments to internal/attachment
kind: refactor
priority: medium
effort: m
status: done
---

## Summary

Move `Workspace.AttachFile` and `Workspace.ListAttachments` (~130 LOC in
`internal/workspace/attachment.go` + associated types) into a new
`internal/attachment` package. CLI consumes the new package directly via its
wiring helper; workspace methods are deleted.

First of three sequential lifts:
1. **TKT-2W0X** (this) — attachment (smallest, ~130 LOC)
2. **TKT-RT01** (filed) — renametype
3. **TKT-AN01** (filed) — analysis (largest, ~450 LOC + types)

Split into three PRs so each is reviewable independently. After all three land,
`cliAnalyze` interface drops its workspace.* type leaks and `internal/cli` stops
importing `internal/workspace` from any production file.

## In scope

- New `internal/attachment` package:
  - Types: `AttachmentInfo`, `AttachResult` (moved verbatim).
  - `attachment.Service` with constructor `attachment.New(deps)` taking Store, Meta, EntityManager. Validates required deps per CLAUDE.md.
  - Methods: `Attach(entityID, filePath, property)` and `List(entityID)` — same behavior as workspace counterparts.
  - Helpers (`findFileProperty`, `contentTypeForName`) move with implementation.
- `internal/cli/cli_wiring.go::cliAnalyze` interface:
  - `AttachFile` / `ListAttachments` method signatures change from `*workspace.AttachResult` / `[]workspace.AttachmentInfo` to `*attachment.AttachResult` / `[]attachment.AttachmentInfo`.
  - `cliServices` forwarders point at the new package's Service.
  - Note: `attach.go` / `attachments.go` call sites only reference the result types via interface; no subcommand changes needed.
- `internal/workspace/attachment.go` DELETED.
- Tests follow the implementation: workspace tests for these methods move to `internal/attachment` (or are deleted if they overlap with new tests).
- `.go-arch-lint.yml`: add `internal/attachment` component; CLI gains dep; workspace drops the file's dependencies.

## Out of scope

- `RenameEntityType` and analysis facades — separate tickets.
- Changes to attachment storage semantics or fsstore's `AttachFile` method.
- Renaming the methods (keep `AttachFile` / `ListAttachments` for API stability through the transition).

## Depends on

- TKT-0SP1 (PR #724) — CLI bundle's `cliAnalyze` interface must already be in place. Stacked on TKT-0SP1's branch.

## Why

Attachment is the smallest facade — proving the lift pattern on a 130-LOC slice
before doing the larger ones reduces risk. After this PR lands, `cliAnalyze`'s
workspace.* type leaks drop from 7 to 5; each subsequent lift removes more.

## Risks

- Tests: `internal/workspace/attachment_test.go` (if it exists) needs to move. Verify before deletion.
- Constructor wiring: the attachment Service needs Store + Meta + EntityManager. CLI's wiring helper already has all three.
- Coverage floors in `.testcoverage.yml`: new package gets a floor; workspace's coverage may shift.
