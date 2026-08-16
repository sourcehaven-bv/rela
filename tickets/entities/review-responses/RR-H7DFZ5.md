---
id: RR-H7DFZ5
type: review-response
title: 'export and analyze_validations are ungated whole-graph reads; analyze_validations executes Lua on the read path despite AC #7''s Lua-free claim'
finding: 'tools_export.go:34 dumps every entity with full properties and every relation via unfiltered ListEntities/ListRelations. tools_analysis.go:304 calls deps.Validator.CheckRule, whose candidate set is ungated AND which executes operator-authored lua_file: rules on the read path — so AC #7''s test (no lua_* in tools/list) passes while Lua still runs remotely. Deps.Validator was absent from the plan''s read inventory entirely.'
severity: significant
resolution: 'Both real halves fixed. (1) Validator candidate set: appbuild.Services.GatedReads() builds a SECOND validator over the gated reader (validator.New already takes a narrow EntityLister for exactly this, per its godoc citing TKT-3FL2S6), and mcp_wiring uses it instead of Services.Validator(), which stays raw for its unattended callers. (2) export now reads through the gated Deps.Store like every other surface. The Lua half was withdrawn by the ticket owner: the CLAUDE.md read-path rule targets unbounded per-record work on hot list views, not a bounded operator-authored validation run the caller requested - so analyze_validations keeps running lua_file: rules, and AC #7 stays ''no lua_* tools remotely''.'
status: addressed
---

## Finding

Two more surfaces missing from the plan's read inventory.

### `export` — highest-yield tool on the surface

`internal/mcp/tools_export.go:27-40`: `ListEntities` with an empty
`store.EntityQuery{}` when no type is given, plus `ListRelations` over
everything, emitting full `properties`. `export(format=json)` is a complete
graph dump. It appears nowhere in the plan's inventory or the AC #3 matrix.

### `analyze_validations` — ungated candidates AND remote Lua execution

`internal/mcp/tools_analysis.go:301-312` calls `s.deps.Validator.CheckRule(ctx,
rule)` and returns violating entity **ids**. Two problems:

1. **Ungated candidate set.** `validator.GenericValidator` reads through an
`EntityLister` (`internal/validator/validator.go:86-95`), and
`internal/cli/mcp_wiring.go` supplies `svc.Validator()` built over the raw
store. The gated composition already exists elsewhere — the godoc notes analyze
wires a per-principal gated reader so a rule's candidate set is the requester's
visible slice (TKT-3FL2S6). MCP does not.
2. **It executes Lua on the read path.** `validator.New(r, meta, deps
lua.ReadDeps)` runs `lua_file:` rules (`validator.go:103`;
`RuleResult.ScriptErrors []*lua.ScriptError`). The ticket's headline safety
decision is that Lua is not exposed remotely (AC #7) — but AC #7's test only
asserts `lua_*` are absent from `tools/list`. Lua still executes remotely via
`analyze_validations`, so **the test passes while the claim is false**.

`Deps.Validator` was not in the plan's inventory at all (which counted only
Store/Tracer/Searcher).

This also touches the CLAUDE.md rule "Don't run user-supplied Lua on the read
path." Operator-authored bounded rules may well be acceptable — but it must be a
*stated decision*, not an accident, and the ticket currently asserts the
opposite.

## Resolution required

1. Exclude `export` and `analyze_validations` from the remote tool set, or gate
both (validator needs the per-principal gated `EntityLister`).
2. Restate AC #7: it must assert **no Lua executes** on the remote transport,
not merely that `lua_*` are absent from `tools/list`.
3. Rebuild the read inventory to include `Deps.Validator`.
