---
id: PLAN-mi7j
status: done
title: 'Planning: Add Lua scripting support via gopher-lua'
type: planning-checklist
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**IN scope:**
- Lua runtime integration via gopher-lua
- New `internal/lua` package with rela bindings
- CLI command `rela script <file.lua> [args...]`
- Imperative output API (`rela.output()`, `rela.write_file()`)
- Entity/relation query functions exposed to Lua
- Full Lua stdlib (security not a concern per user)
- Scripts located in project `scripts/` directory (convention)

**OUT of scope:**
- MCP tool for script execution (future ticket)
- Watch mode / live reload
- Script caching / precompilation
- Inline scripts in views.yaml
- Custom metamodel validation via Lua (future use case)

**Acceptance Criteria:**

1. **AC1**: `rela script test.lua` executes a Lua script and exits with code 0 on success
   - Test: Create script that calls `rela.output({status="ok"})`, verify JSON stdout

2. **AC2**: Scripts can query entities via `rela.get_entity(id)` returning a Lua table
   - Test: Script fetches known entity, verifies id/type/properties are accessible

3. **AC3**: Scripts can list entities via `rela.list_entities(type, filter?)` 
   - Test: Script lists all "ticket" entities, filters by `status=done`, verifies count

4. **AC4**: Scripts can query relations via `rela.get_relations(from?, type?, to?)`
   - Test: Script gets all "implements" relations, verifies from/type/to fields

5. **AC5**: Scripts can write files via `rela.write_file(path, content)`
   - Test: Script writes temp file, verify file exists with correct content

6. **AC6**: Script errors are surfaced with file/line information
   - Test: Script with syntax error shows "script.lua:5: unexpected symbol"

7. **AC7**: Script arguments accessible via `rela.args` table
   - Test: `rela script test.lua foo bar` → `rela.args = {"foo", "bar"}`

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

**Library: gopher-lua (github.com/yuin/gopher-lua)**
- Pros: Pure Go, no CGO, Lua 5.1 compatible, well-maintained, good docs
- Pros: Easy to embed, simple function registration API
- Cons: Lua 5.1 only (not 5.3/5.4) - acceptable for our needs
- Decision: **Use this** - it's the de facto standard for Go+Lua

**Codebase patterns:**
- CLI command pattern: `internal/cli/export.go` - file processing, JSON output
- Workspace access: `internal/cli/root.go:30-40` - shared `ws`, `meta`, `out` vars
- Entity filtering: `internal/filter/filter.go` - `Parse()`, `MatchAll()` for filters
- Output: `internal/output/output.go` - `WriteJSON()`, encoder patterns

**Reference implementations:**
- Similar approach in other tools: Neovim's Lua API, Redis Lua scripting
- Pattern: Register module table, expose typed functions, return Lua tables

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Package Structure

```
internal/lua/
├── runtime.go      # Lua VM lifecycle, script execution
├── bindings.go     # rela.* function implementations  
├── convert.go      # Go ↔ Lua type conversion helpers
└── runtime_test.go # Unit tests
```

### Core Types

```go
// Runtime wraps gopher-lua VM with rela bindings
type Runtime struct {
    L       *lua.LState
    ws      *workspace.Workspace
    meta    *metamodel.Metamodel
    stdout  io.Writer  // for rela.output()
}

// New creates a Runtime with rela bindings registered
func New(ws *workspace.Workspace, meta *metamodel.Metamodel, stdout io.Writer) *Runtime

// RunFile executes a Lua script file with arguments
func (r *Runtime) RunFile(path string, args []string) error

// Close releases Lua VM resources
func (r *Runtime) Close()
```

### Lua API Surface

```lua
-- Entity access
entity = rela.get_entity("TKT-001")        -- returns table or nil
entities = rela.list_entities("ticket")    -- returns array of tables
entities = rela.list_entities("ticket", "status=done")  -- with filter

-- Relation access  
relations = rela.get_relations()                    -- all relations
relations = rela.get_relations({from="TKT-001"})    -- filter by from
relations = rela.get_relations({type="implements"}) -- filter by type

-- Graph traversal
trace = rela.trace_from("TKT-001", 2)  -- trace outgoing, max depth 2
trace = rela.trace_to("TKT-001")       -- trace incoming, unlimited

-- Output
rela.output(data)                -- JSON encode to stdout
rela.write_file("out.json", json.encode(data))  -- write file

-- Context
rela.args          -- table of CLI arguments
rela.project_root  -- project root path
```

### Entity Table Structure (Lua)

```lua
entity = {
    id = "TKT-001",
    type = "ticket",
    properties = {
        title = "Fix bug",
        status = "done",
        priority = "high"
    },
    content = "Markdown body..."
}
```

### CLI Command

```go
// internal/cli/script.go
var scriptCmd = &cobra.Command{
    Use:   "script <file.lua> [args...]",
    Short: "Execute a Lua script against the graph",
    Args:  cobra.MinimumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        runtime := lua.New(ws, meta, os.Stdout)
        defer runtime.Close()
        return runtime.RunFile(args[0], args[1:])
    },
}
```

**Alternatives considered:**

1. **JavaScript via goja** - Rejected: Larger runtime, more complex API
2. **Starlark (Python subset)** - Rejected: Less familiar syntax, fewer users
3. **Expression language only** - Rejected: Too limited for complex transformations
4. **Return value instead of imperative** - Rejected per user: less flexible

**Files to modify:**

| File | Change |
|------|--------|
| `internal/lua/runtime.go` | NEW: Lua VM wrapper |
| `internal/lua/bindings.go` | NEW: rela.* function implementations |
| `internal/lua/convert.go` | NEW: Go ↔ Lua type conversion |
| `internal/lua/runtime_test.go` | NEW: Unit tests |
| `internal/cli/script.go` | NEW: CLI command |
| `.go-arch-lint.yml` | ADD: lua component and deps |
| `go.mod` | ADD: gopher-lua dependency |

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On Invalid |
|-------|--------|------------|------------|
| Script path | CLI arg | File exists, readable | Error with path |
| Script args | CLI args | None (passthrough) | N/A |
| Filter expressions | Script | Use existing filter.Parse() | Lua error |
| Entity IDs | Script | Graph lookup (not found = nil) | Return nil |
| Output file path | Script | Relative to project root | Error |

**Security-Sensitive Operations:**

Per user: "Security is not of concern at this point (tools run locally with trusted data, rela already has options to execute commands)."

- Full Lua stdlib enabled (including `io`, `os`)
- File writes allowed anywhere (user's responsibility)
- No sandboxing required

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Approach |
|----|------|----------|
| AC1 | Script execution | Unit: RunFile returns nil on success |
| AC2 | get_entity | Unit: Mock workspace, verify table fields |
| AC3 | list_entities | Unit: Filter expression passed to filter.Parse |
| AC4 | get_relations | Unit: Verify from/type/to filtering |
| AC5 | write_file | Unit: Write to temp dir, verify content |
| AC6 | Error reporting | Unit: Syntax error includes line number |
| AC7 | Args passing | Unit: Verify rela.args table |

**Integration test:**
- Create test project with entities in `internal/lua/testdata/`
- Script that queries entities, filters, outputs JSON
- Verify JSON output matches expected

**Edge Cases:**

| Case | Expected Behavior |
|------|-------------------|
| Empty script | No output, exit 0 |
| Script not found | Error: "script not found: path" |
| Entity not found | `rela.get_entity()` returns nil |
| Invalid filter syntax | Lua error with filter parse message |
| Empty entity list | Returns empty table `{}` |
| Circular relations in trace | Handled by existing trace logic |
| Very large output | Works (streaming not required for v1) |
| Unicode in properties | Preserved correctly |

**Negative Tests:**

| Test | Input | Expected |
|------|-------|----------|
| Missing script | `rela script` | Usage error |
| Script syntax error | `print(` | Error with line number |
| Runtime error | `error("boom")` | Error with message |
| Unknown function | `rela.foo()` | "attempt to call nil" |
| Wrong arg type | `rela.get_entity(123)` | Type error message |

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| gopher-lua API learning curve | Low | Medium | Good docs, simple use case |
| Type conversion complexity | Medium | Medium | Start with basic types, iterate |
| Performance with large graphs | Low | Low | Lua is fast, workspace caches |

**Effort:** L (large) - New package, new dependency, multiple binding functions

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: implementation complete)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: implementation complete)

**Design Review Findings:** N/A - retroactive checklist completion.
