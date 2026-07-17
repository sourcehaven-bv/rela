---
id: TKT-45QYI
type: ticket
title: Decompose cli.cliServices bundle into read/write field bundles + direct service bindings
kind: refactor
priority: high
effort: m
status: done
---

## What

`internal/cli`'s `cliServices` was a container pretending to be a service: 30
exported methods, all pure delegation (15 to `appbuild.Services`, 11 to
`analysis.Service`, 2 to `attachment.Service`, 1 to `renametype.Service`). PR
#1142 (TKT-BW6UUL) added `Audit()`, pushing it past its grandfathered plimsoll
load line of 29 and breaking CI on develop. Instead of bumping the pin, the
god-object was removed.

## How

- `readServices` (8 fields) / `writeServices` (embeds read + 6 fields) —
plain field structs, zero methods, same shape as `lua.ReadDeps`/`WriteDeps`.
- kong binds both bundles plus `*analysis.Service`, `*attachment.Service`,
`*renametype.Service` directly; each command's `Run` binds only what it uses
(six analyze subcommands turned out to need only the analyzer).
- Scheduler command takes `scheduler.WorkspaceProvider`, supplied at the
wiring site via `BindTo` with `appbuild.Services` (compile-time assertion added
in kong.go since kong checks assignability only at runtime).
- Grandfather directive `//plimsoll:max-exported-methods=29` deleted.

43 files, +365/−395. Verified: full test suite, golangci-lint, plimsoll,
arch-lint, coverage floors, all three build tags, CLI smoke test.

Note: the `cli.CLI` kong root struct's `//plimsoll:max-fields=45` directive
remains — growth there is structural (one field per subcommand), tracked in
TKT-N0IKN9.
