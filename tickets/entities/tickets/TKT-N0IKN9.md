---
id: TKT-N0IKN9
type: ticket
title: Decompose god-object types flagged by plimsoll (App, Runtime, FSStore, Server, CLI)
kind: refactor
priority: medium
effort: l
status: backlog
---

We added the [plimsoll](https://github.com/sourcehaven-bv/plimsoll) god-object
linter (caps method/exported-field count per type) to CI. Five existing types
are over the load line and are grandfathered with `//plimsoll:max-*` directives
pinned to their CURRENT count, so they can't grow. This ticket tracks ratcheting
those numbers down.

## Why this happened

Adding the Nth method to an existing struct is frictionless; spinning up a
focused new type is work — so every feature accreted onto the nearest big type.
Nothing failed when `App` grew its 200th method. (Root cause from the sync
read-ACL review: the same "convention, not enforcement" gap.) plimsoll is the
structural brake; this ticket is the cleanup of the debt that accrued before it.

## Offenders (cap pinned at current count)

| Type | Surface | File |
| --- | --- | --- |
| `dataentry.App` | 226 methods | `internal/dataentry/app.go` |
| `lua.Runtime` | 119 methods | `internal/lua/runtime.go` |
| `fsstore.FSStore` | 84 methods | `internal/store/fsstore/fsstore.go` |
| `mcp.Server` | 47 methods | `internal/mcp/server.go` |
| `cli.CLI` | 37 exported fields | `internal/cli/kong.go` |

## Approach

Per type: extract cohesive responsibilities into their own types with narrow,
injected dependencies (consumer-side interfaces per the project rules), then
lower the `//plimsoll:max-*` number. `App` is the priority — its decomposition
into an `api` package with a gated read interface also closes the read-ACL
class of bug (a route package that never holds `store.Store` can't skip the
read gate). `CLI` growth is structural (kong binds one field per subcommand);
revisit grouping into sub-structs but it may stay grandfathered.

## Progress

- `dataentry.App` decomposition tracked in sub-arc TKT-N26KLB (227 → ~166
  methods; the `visibleReader` ACL seam landed), remainder in TKT-R68TV8.
- `metamodel.Metamodel`: the attachment-scan accessors (`ScanCommandFor`,
  `HasUnconfiguredScan`) were moved behind a focused `AttachmentPolicy` view
  rather than widening the metamodel's public surface — the plimsoll line held
  at 30 instead of being bumped when the attachment feature landed. This is the
  ratchet working as intended: the linter forced the extraction at write time.
