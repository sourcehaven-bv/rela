<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Lua Scripting

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

# Run an interactive flow (with user prompts)
rela flow scripts/create-ticket.lua
```

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

## Interactive Flows

Interactive flows allow Lua scripts to present forms to users and receive input. The script
suspends at each form, waits for user input, then continues execution. This enables multi-step
wizards, guided entity creation, and interactive data collection.

### Running a Flow

```bash
rela flow scripts/create-ticket.lua
```

### Executable Scripts

Flow scripts can be made directly executable with a shebang:

```lua
#!/usr/bin/env -S rela flow
-- Your flow script here

local event = rela.flow.emit({
    type = "form",
    -- ...
})
```

Then run directly:

```bash
chmod +x scripts/create-ticket.lua
./scripts/create-ticket.lua
```

### The emit() Function

Use `rela.flow.emit()` to present a form and wait for user response:

```lua
local event = rela.flow.emit({
    type = "form",
    title = "Create Ticket",
    fields = {
        {name = "title", type = "text", required = true},
        {name = "priority", type = "select",
         options = {{"high", "High"}, {"medium", "Medium"}, {"low", "Low"}}},
    },
    actions = {
        {"submit", "Create"},
        {"cancel", "Cancel"},
    },
})

if event.action == "cancel" then
    return
end

rela.create_entity("ticket", {
    title = event.data.title,
    priority = event.data.priority,
})
```

### Form Specification

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `type` | string | yes | Must be `"form"` |
| `title` | string | no | Form title |
| `description` | string | no | Help text shown below title |
| `fields` | array | yes | Field definitions (see below) |
| `actions` | array | yes | Action buttons (see below) |

### Field Types

| Type | Options | Returns |
|------|---------|---------|
| `text` | `required`, `default`, `placeholder`, `lines` | string |
| `select` | `options`, `required`, `default` | selected value |
| `multi-select` | `options`, `required`, `default` | array of values |
| `boolean` | `default` | true/false |
| `number` | `required`, `default`, `min`, `max`, `step` | number |
| `date` | `required`, `default`, `min`, `max` | "YYYY-MM-DD" |
| `markdown` | `content`, `label` | (display only, no data) |

### Field Properties

| Property | Type | Description |
|----------|------|-------------|
| `name` | string | Field identifier (required, except for markdown) |
| `type` | string | Field type (required) |
| `label` | string | Display label (defaults to title-cased name) |
| `content` | string | Markdown content (required for markdown fields) |
| `required` | boolean | Whether field is required |
| `default` | varies | Default value |
| `placeholder` | string | Placeholder text (text fields only) |
| `lines` | number | Number of lines for textarea (text fields only) |
| `options` | array | Options for select/multi-select: `{{"value", "Label"}, ...}` |
| `min`, `max` | number/string | Bounds for number or date fields |
| `step` | number | Step increment for number fields |

### Markdown Fields

Markdown fields display formatted text within a form. They don't collect user input—use them to
provide instructions, context, or visual separators between input fields:

```lua
fields = {
    {type = "markdown", content = "## Instructions\nPlease fill out the form below."},
    {name = "title", type = "text", required = true},
    {type = "markdown", content = "---\n*Additional options:*"},
    {name = "priority", type = "select", options = {{"high", "High"}, {"low", "Low"}}},
    {type = "markdown", label = "Note", content = "Fields marked with * are required."},
}
```

Markdown fields support:

- `content` (required): The markdown text to display
- `label` (optional): A title shown above the content

### Actions

Actions are defined as tuples: `{id, label}` or `{id, label, style}`:

```lua
actions = {
    {"submit", "Create"},           -- Default style
    {"cancel", "Cancel", "warning"}, -- Warning style
}
```

Styles: `primary`, `warning`, `danger`

### Event Response

When a form is submitted, `emit()` returns an event table:

```lua
{
    action = "submit",  -- The action ID the user chose
    data = {            -- Field values (for submit-like actions)
        title = "My Ticket",
        priority = "high",
    },
}
```

### Multi-Step Flows

Scripts can present multiple forms in sequence:

```lua
local data = {}

-- Step 1: Basic info
local e1 = rela.flow.emit({
    type = "form",
    title = "Step 1: Basic Info",
    fields = {
        {name = "title", type = "text", required = true},
        {name = "kind", type = "select",
         options = {{"bug", "Bug"}, {"feature", "Feature"}}},
    },
    actions = {{"next", "Next"}, {"cancel", "Cancel"}},
})
if e1.action == "cancel" then return end
data.title = e1.data.title
data.kind = e1.data.kind

-- Step 2: Details
local e2 = rela.flow.emit({
    type = "form",
    title = "Step 2: Details",
    fields = {
        {name = "description", type = "text", lines = 5},
        {name = "priority", type = "select",
         options = {{"high", "High"}, {"medium", "Medium"}, {"low", "Low"}}},
    },
    actions = {{"back", "Back"}, {"submit", "Create"}, {"cancel", "Cancel"}},
})
if e2.action == "cancel" then return end
if e2.action == "back" then
    -- Handle back navigation (use goto or restructure as loop)
end
data.description = e2.data.description
data.priority = e2.data.priority

rela.create_entity("ticket", data)
```

### Error Handling

Validation errors (invalid form spec) raise Lua errors. User cancellation is handled via
actions, not errors:

```lua
-- Handle cancel action explicitly
if event.action == "cancel" then
    print("User cancelled")
    return
end
```

Transport errors (e.g., terminal not interactive) also raise Lua errors.

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

### Flow Functions

| Function | Description | Returns |
|----------|-------------|---------|
| `rela.flow.emit(screen)` | Present form, wait for user input | event table |

See [Interactive Flows](#interactive-flows) for detailed documentation.

### Utility Functions

| Function | Description | Returns |
|----------|-------------|---------|
| `rela.days_since(date)` | Days between date and today | number (-1 if invalid) |
| `rela.sort_entities(list, prop, dir?)` | Sort entities by property | sorted table |

### Context

| Variable | Description |
|----------|-------------|
| `rela.project_root` | Absolute path to project root |
| `rela.args` | Script arguments (table) |
| `rela.today` | Current date as "YYYY-MM-DD" |

## Entity Structure

Entities returned by query functions have this structure:

```lua
{
    id = "TKT-001",
    type = "ticket",
    content = "Markdown body...",
    mod_time = "2024-01-15T10:30:00Z",  -- RFC3339 timestamp
    properties = {
        title = "Fix login bug",
        status = "open",
        priority = "high"
    }
}
```

Entities also have helper methods:

- `entity:prop(name, default)` - Get property with fallback default
- `entity:strip_prefix()` - Get ID without type prefix (e.g., "001" from "TKT-001")

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
