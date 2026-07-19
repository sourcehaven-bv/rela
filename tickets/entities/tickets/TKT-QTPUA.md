---
id: TKT-QTPUA
type: ticket
title: Extract dataentry attachment route cluster into attachmentHandler
kind: refactor
priority: medium
effort: s
status: done
---

Sub-ticket of the [[TKT-R68TV8]] `dataentry.App` decomposition arc (M5.2 tail).
Follows [[TKT-VG9P1]] (sync) and [[TKT-KUFLD]] (command).

## What

Moved the 12-method attachment surface (`handlers_attachment.go`) off `App`: 8
methods onto a new `attachmentHandler`, 4 demoted to package-level functions
(`probeAttachmentCommands`, `writeAttachmentTooLarge`,
`writeAttachmentWriteError`, `isFileProperty` — none used App state).

- **`App`: 143 → 131 methods** — `//plimsoll:max-methods` directive ratcheted.
- Holds the full `store.Store` + `entitymanager.EntityManager` because
`attachment.New` (the shared HTTP/CLI write-policy service) requires both.
- The swappable collaborators (`acl`, audit sink, field resolver, command
runner) are **closures over App** — the attachment ACL tests reassign
`app.acl`/`app.attachmentRunner` after construction (same rationale as
`affordanceService`).
- `writeMu` shared by **pointer**: uploads/deletes mutate the owning entity's
property, so they serialize with every other mutation handler.
- Capture-once improvement: the PUT path now reuses its `Schema` snapshot for
the wire serialization (`s.Meta`) instead of a second `a.Meta()` load; the
GET/preflight property checks take the snapshot explicitly.

## Drive-by

Bumped `internal/cli/cli_wiring.go` `//plimsoll:max-exported-methods` 29 → 30:
PR #1142 added `cliServices.Audit()` without adjusting the pin, leaving develop
red on the God-object lint for every PR.

## Verification

- `go test -race ./internal/dataentry/...`
- Builds across default / `postgres` / `memorybackend`
- `just plimsoll`, `just arch-lint`, `golangci-lint` (0 issues), `gofmt`
