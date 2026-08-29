---
id: TKT-YUETL7
type: ticket
title: Extract typeResolver + trace/export handlers off mcp.Server (plimsoll ratchet 49 → 38)
kind: refactor
priority: medium
effort: m
status: done
---

Sub-ticket of [[TKT-N0IKN9]], opening the `mcp.Server` arc. Server sits at
`//plimsoll:max-methods=49` with only 4 struct fields; every tool handler is a
`*Server` method reaching through `s.deps.X` — there is no per-cluster
injection, which is precisely the seam this extraction installs.

## What

**Pure structural extraction, no behavior change, no wire-visible change.**

- `typeResolver` — tiny value type over `deps.Meta` taking the three shared
helpers from `tools_helpers.go` (`resolveType`, `resolveEntityType`,
`validatePropertyNames`). The cross-cluster substrate every later extraction
needs (the mdHelpers shape: one field, state-free helpers). −3.
- `traceHandler` — `tools_trace.go` (4 methods: `handleTraceFrom`,
`handleTraceTo`, `handleTrace`, `handleFindPath`) over `{store GraphReader;
tracer}`. −4.
- `exportHandler` — `tools_export.go` (4 methods: `handleExport`, `exportJSON`,
`exportYAML`, `exportCSV`) over `{store GraphReader}` — the cleanest cut in the
package (one dep). −4.

`registerTools` stays on Server; the affected `AddTool` lines re-point to
`s.trace.handleX` / `s.export.handleX`. Ratchet directive 49 → 38 (under the 40
load line — but the directive stays until the arc's later steps decide whether
Server can drop it entirely).

Also fix the doc drift: `docs/architecture/consumer-side-interfaces.md` still
names `mcp.Services`; the type is now the `Deps` capability bundle.

## Done when

plimsoll with lowered directive, full test suite green (dispatch_test.go and
golden_test.go are the behavior guards), arch-lint/comment-lint/lint clean.
