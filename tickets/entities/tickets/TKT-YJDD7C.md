---
id: TKT-YJDD7C
type: ticket
title: 'cli.CLI: group subcommands into embed:"" command-group structs and replace the package-level invocation globals with a bound Globals value'
kind: refactor
priority: low
effort: m
tags:
    - tech-debt
status: ready
---

Sub-ticket of [[TKT-N0IKN9]] — `cli.CLI` (47 exported fields, directive at
`internal/cli/kong.go:70`). Two steps; step 1 is cheap, step 2 is the real
design fix.

## Step 1 — command groups via `embed:""` (47 → 7 fields, directive deleted)

Verified empirically against kong v1.16.1 and plimsoll v0.2.0:
- `embed:""` splices the embedded struct's fields into the PARENT's field
list (kong build.go:88-90) — `rela show` stays `rela show`, `ktx.Command()` is
unchanged, so `requiresProject` (kong.go:256-277) and its token parsing keep
working. `cmd:""` on a group struct would create a node (`rela group show`) — do
NOT use it.
- plimsoll counts the syntactic struct literal only (analyzer.go:255-278); an
embedded exported type counts as 1.
- Global flags embed too, keeping `--project`/`-v`/`-o`/`-q` at the root with
`env:`/`short:` tags intact.

```go
type CLI struct {
    Globals        `embed:""`   // Project, Output, Verbose, Quiet
    EntityCmds     `embed:""`   // Show List Create Update Delete Link Unlink Renumber
    GraphCmds      `embed:""`   // Trace Graph Analyze
    ContentCmds    `embed:""`   // Export Render Import Sync Fmt Normalize Rename Gc Attach Attachments Detach
    VersioningCmds `embed:""`   // History Restore RelationHistory RelationRestore HistoryPurge RelationHistoryPurge
    ProjectCmds    `embed:""`   // Schema Template Validate Migrate ACL Secrets Init
    RuntimeCmds    `embed:""`   // Script Scheduler Flow Mcp Db Apps Version Completion
}
```

Each group lives in its own file beside its commands — the versioning six (all
postgres-only) become one greppable cluster instead of six fields in a 50-line
block. Keep the doc comment at kong.go:53-68 explaining that the struct is a
kong format mirror.

## Step 2 — the actual smell: package-level mutable invocation state

`out`, `verbose`, `quiet`, `outputFormat`, `projectPath` (kong.go:44-49) are
package globals copied out of the four flag fields in `runKong`
(kong.go:174-177) and read implicitly by every subcommand. kong.go:6-10
acknowledges it. Thread a `*Globals` (or an `invocation` value built from it)
through `ktx.Bind(...)` so `Run` methods receive it as a parameter, and delete
the globals. This is a larger, mechanical change across the `*Cmd.Run`
signatures; do it as its own commit (or PR) after step 1, and it is the part
that makes the CLI testable without process-global state.

## Note, not in scope

`plimsoll ./...` also reports `package cli has 91 exported types, over the load
line of 40` (`max-exported-types`, not gated in CI today). Grouping fields does
not change that; it is the more accurate signal about this package and should be
discussed separately.
