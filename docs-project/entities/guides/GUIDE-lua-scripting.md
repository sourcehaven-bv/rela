---
audience: advanced
id: GUIDE-lua-scripting
order: 10
status: published
summary: Programmable automation with embedded Lua
title: Lua Scripting
type: guide
---

Rela includes an embedded Lua scripting runtime for programmable access to the entity graph.
Scripts can query, mutate, analyze, and export data—enabling automation beyond what's possible
with the CLI or MCP tools alone.

## Quick Start

### Via MCP (AI Assistants)

```lua
-- lua_eval: Execute inline Lua code
local tickets = rela.list_entities("ticket", "status=open")
rela.output({count = #tickets})
```

### Via CLI

```bash
# Run a script file
rela lua scripts/report.lua

# Pass arguments
rela lua scripts/migrate.lua --from=v1 --to=v2
```

### Shebang Support

Scripts can include a shebang line for direct execution from the command line:

```lua
#!/usr/bin/env -S rela script
local entities = rela.list_entities("ticket", "status=open")
rela.output({count = #entities})
```

```bash
chmod +x scripts/report.lua
./scripts/report.lua
```

The shebang line is automatically stripped before execution. Line numbers in error
messages remain accurate.

### Script Location

Scripts executed via `lua_run` must be in the `scripts/` directory:

```text
project/
├── scripts/
│   ├── report.lua
│   └── utils/
│       └── helpers.lua
├── entities/
└── metamodel.yaml
```

## API Reference

### Query Functions

| Function | Description | Returns |
|----------|-------------|---------|
| `rela.get_entity(id)` | Get entity by ID | table or nil |
| `rela.list_entities(type, filter?)` | List entities of a type | table (array) |
| `rela.search(query, limit?)` | Full-text search | table (array) |
| `rela.get_relations(opts?)` | Get relations with filters | table (array) |
| `rela.trace_from(id, depth?)` | Trace outgoing dependencies | table (tree) |
| `rela.trace_to(id, depth?)` | Trace incoming dependencies | table (tree) |
| `rela.find_path(from, to)` | Find shortest path | table (array) or nil |

### Mutation Functions

| Function | Description | Returns |
|----------|-------------|---------|
| `rela.create_entity(type, props, content?, id?)` | Create entity | table |
| `rela.update_entity(id, props, content?)` | Update entity | table |
| `rela.delete_entity(id, cascade?)` | Delete entity | boolean |
| `rela.create_relation(from, type, to)` | Create relation | table |
| `rela.delete_relation(from, type, to)` | Delete relation | boolean |
| `rela.refresh()` | Reload graph from disk | boolean |

### Schema Functions

| Function | Description | Returns |
|----------|-------------|---------|
| `rela.get_entity_types()` | Get all entity type definitions | table |
| `rela.get_relation_types()` | Get all relation type definitions | table |

### Output Functions

| Function | Description |
|----------|-------------|
| `rela.output(data)` | Output data as JSON to stdout |
| `rela.write_file(path, content)` | Write file to `output/` directory |

### Context

| Variable | Description |
|----------|-------------|
| `rela.project_root` | Absolute path to project root |
| `rela.args` | Script arguments (table) |

## Entity Structure

Entities returned by query functions have this structure:

```lua
{
    id = "TKT-001",
    type = "ticket",
    content = "Markdown body...",
    properties = {
        title = "Fix login bug",
        status = "open",
        priority = "high"
    }
}
```

## Filter Expressions

The `list_entities` function accepts filter expressions:

```lua
-- Single value
rela.list_entities("ticket", "status=open")

-- Multiple values (OR)
rela.list_entities("ticket", "status=open,in-progress")

-- Not equal
rela.list_entities("ticket", "priority!=low")
```

## Examples

### Generate a Report

```lua
-- Export open tickets as markdown
local tickets = rela.list_entities("ticket", "status=open")

local lines = {"# Open Tickets", ""}
for _, t in ipairs(tickets) do
    table.insert(lines, string.format("- **%s**: %s", t.id, t.properties.title))
end

rela.write_file("open-tickets.md", table.concat(lines, "\n"))
rela.output({exported = #tickets})
```

### Bulk Update

```lua
-- Close all tickets in a completed sprint
local sprint = rela.args[1] or "2024-Q1"
local tickets = rela.list_entities("ticket", "sprint=" .. sprint)

local updated = 0
for _, t in ipairs(tickets) do
    if t.properties.status ~= "done" then
        rela.update_entity(t.id, {status = "cancelled"})
        updated = updated + 1
    end
end

rela.output({sprint = sprint, cancelled = updated})
```

### Find Orphans

```lua
-- Find entities with no relations
local orphans = {}
local types = rela.get_entity_types()

for name, _ in pairs(types) do
    local entities = rela.list_entities(name)
    for _, e in ipairs(entities) do
        local from = rela.trace_from(e.id, 1)
        local to = rela.trace_to(e.id, 1)
        if #from.children == 0 and #to.children == 0 then
            table.insert(orphans, {
                id = e.id,
                type = e.type,
                title = e.properties.title
            })
        end
    end
end

rela.output(orphans)
```

### Create Related Entities

```lua
-- Create test cases from feature acceptance criteria
local feature_id = rela.args[1]
local feature = rela.get_entity(feature_id)

if not feature then
    error("Feature not found: " .. feature_id)
end

local created = {}
for line in feature.content:gmatch("[^\n]+") do
    if line:match("^%s*[-*]") then
        local criterion = line:gsub("^%s*[-*]%s*", "")
        local test = rela.create_entity("test-case", {
            title = "Verify: " .. criterion,
            status = "pending"
        })
        rela.create_relation(test.id, "verifies", feature_id)
        table.insert(created, test.id)
    end
end

rela.output({feature = feature_id, tests_created = created})
```

### Traceability Matrix

```lua
-- Generate requirement-to-test coverage matrix
local reqs = rela.list_entities("requirement")
local matrix = {}

for _, req in ipairs(reqs) do
    local entry = {
        id = req.id,
        title = req.properties.title,
        tests = {},
        covered = false
    }
    
    local trace = rela.trace_from(req.id, 2)
    for _, child in ipairs(trace.children) do
        if child.type == "test-case" then
            table.insert(entry.tests, child.id)
        end
    end
    
    entry.covered = #entry.tests > 0
    table.insert(matrix, entry)
end

-- Summary
local total = #matrix
local covered = 0
for _, entry in ipairs(matrix) do
    if entry.covered then covered = covered + 1 end
end

rela.output({
    matrix = matrix,
    summary = {
        total = total,
        covered = covered,
        uncovered = total - covered,
        coverage_pct = math.floor(covered / total * 100)
    }
})
```

## Security

The Lua runtime is sandboxed for security:

### Available Libraries

- `string` - String manipulation
- `table` - Table operations
- `math` - Mathematical functions
- `coroutine` - Coroutine support
- Base functions: `print`, `pairs`, `ipairs`, `type`, `tostring`, `tonumber`, `error`, `pcall`,
  `assert`, `select`, `next`, `unpack`

### Unavailable (Security)

- `io` - No direct file system access
- `os` - No system commands
- `debug` - No runtime introspection
- `load`, `loadfile`, `dofile`, `loadstring` - No dynamic code loading
- `rawget`, `rawset`, `getmetatable`, `setmetatable` - No metatable manipulation

### File Writing

Files can only be written to the `output/` directory:

```lua
-- OK: writes to output/report.txt
rela.write_file("report.txt", "content")

-- OK: writes to output/reports/2024/summary.md  
rela.write_file("reports/2024/summary.md", "content")

-- ERROR: path traversal blocked
rela.write_file("../secret.txt", "content")

-- ERROR: absolute paths blocked
rela.write_file("/etc/passwd", "content")
```

## MCP Tools

Lua scripting is available via MCP tools:

| Tool | Description |
|------|-------------|
| `lua_eval` | Execute inline Lua code |
| `lua_run` | Execute a script from `scripts/` |
| `lua_list` | List available scripts |

### lua_eval

Execute inline Lua code:

```text
lua_eval(code: "rela.output(rela.list_entities('ticket'))")
```

### lua_run

Execute a script file with arguments:

```text
lua_run(path: "report.lua", args: ["2024-Q1"])
```

### lua_list

List available scripts in `scripts/` directory.

## Validation Rules

Lua can be used in metamodel validation rules for complex business logic. See the
[Lua Validation](metamodel.md#lua-validation) section in the Metamodel Reference for details.

```yaml
validations:
  - name: check-coverage
    description: "Components need 80% test coverage"
    entity_type: component
    lua: |
      local cov = entity.properties.coverage
      if cov == nil then return nil end
      local value = tonumber(cov)
      if value and value < 80 then
        return { message = "Coverage is " .. value .. "%, need 80%" }
      end
      return nil
    severity: error
```

Key differences from script execution:

| Feature | Scripts (lua_eval/lua_run) | Validation Rules |
|---------|---------------------------|------------------|
| Workspace access | Full read/write | Read-only |
| Timeout | None | 5 seconds |
| Output | `rela.output()` | Return `nil` (pass) or `{message=...}` (violation) |
| File I/O | `rela.write_file()` | Not available |
