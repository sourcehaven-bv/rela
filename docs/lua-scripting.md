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

### Date Functions

| Function | Description | Returns |
|----------|-------------|---------|
| `rela.date_add(date, offset)` | Add offset to date | date string |
| `rela.date_weekday(date)` | Get weekday name | lowercase string |
| `rela.date_next_weekday(date, day)` | Next occurrence of weekday after date | date string |
| `rela.rrule_next(rrule, after?)` | Next RRULE occurrence (default: after today) | date string or nil |

#### date_add

Add a duration offset to a date. Offsets use `Nd`, `Nw`, `Nm`, `Ny` format. Negative values
are supported.

```lua
rela.date_add("2025-01-15", "7d")   -- "2025-01-22"
rela.date_add("2025-01-15", "2w")   -- "2025-01-29"
rela.date_add("2025-01-15", "1m")   -- "2025-02-15"
rela.date_add("2025-01-15", "1y")   -- "2026-01-15"
rela.date_add("2025-01-15", "-3d")  -- "2025-01-12"
```

#### date_weekday

Returns the lowercase weekday name for a date.

```lua
rela.date_weekday("2025-01-06")  -- "monday"
```

#### date_next_weekday

Returns the next occurrence of the given weekday strictly after the date. If the date is
already that weekday, it advances to the following week.

```lua
rela.date_next_weekday("2025-01-06", "friday")  -- "2025-01-10"
rela.date_next_weekday("2025-01-06", "monday")  -- "2025-01-13" (not same day)
```

#### rrule_next

Computes the next occurrence of an iCal RRULE (RFC 5545) after a given date. If no `after`
date is provided, uses today. Returns `nil` if the rule has no more occurrences.

Accepts both `RRULE:FREQ=...` and bare `FREQ=...` formats.

**Important:** Rules with `INTERVAL` > 1 must include `DTSTART` to anchor the interval
cadence. Without it, the function raises an error.

```lua
-- Next Saturday
rela.rrule_next("FREQ=WEEKLY;BYDAY=SA;DTSTART=20250101T000000Z", "2025-01-06")
-- "2025-01-11"

-- 1st of each month
rela.rrule_next("FREQ=MONTHLY;BYMONTHDAY=1;DTSTART=20250101T000000Z", "2025-01-15")
-- "2025-02-01"

-- Last day of each month
rela.rrule_next("FREQ=MONTHLY;BYMONTHDAY=-1;DTSTART=20250101T000000Z", "2025-01-15")
-- "2025-01-31"

-- Every 2 weeks (INTERVAL > 1 requires DTSTART)
rela.rrule_next("FREQ=WEEKLY;INTERVAL=2;DTSTART=20250106T000000Z", "2025-01-06")
-- "2025-01-20"

-- 1st Saturday every 3 months (INTERVAL > 1 requires DTSTART)
rela.rrule_next("FREQ=MONTHLY;INTERVAL=3;BYDAY=1SA;DTSTART=20250101T000000Z", "2025-01-06")
-- "2025-04-05"

-- Uses today if no after date
rela.rrule_next("FREQ=DAILY")  -- tomorrow's date
```

### AI Functions

The `ai` module provides access to LLM providers (OpenAI, ollama, etc.) configured
via `.rela/ai.yaml`. These functions are available when AI is configured; otherwise
they return a `not_configured` error.

| Function | Description | Returns |
|----------|-------------|---------|
| `ai.chat(opts)` | Chat completion | (result_table, nil) or (nil, err_table) |
| `ai.complete(prompt)` | Single-message chat (convenience) | (string, nil) or (nil, err_table) |
| `ai.embed(input, opts?)` | Compute vector embeddings | (array_of_arrays, nil) or (nil, err_table) |

#### Configuration

Create `.rela/ai.yaml` (gitignored, per-user):

```yaml
base_url: http://127.0.0.1:11434/v1   # required
model: gemma3:12b                      # required, default chat model
embedding_model: nomic-embed-text      # optional, falls back to model
api_key_env: OPENAI_API_KEY            # optional, absent = no auth
timeout_seconds: 60                    # optional, default 30
```

#### ai.chat

Send a chat completion request with full control over messages, model, and parameters.

```lua
local result, err = ai.chat({
  messages = {
    {role = "system", content = "You are concise."},
    {role = "user",   content = "What is 2+2?"},
  },
  model = "gemma3:12b",   -- optional, falls back to config
  temperature = 0,        -- optional; 0 is distinct from unset
  max_tokens = 50,        -- optional
})
if err then
  print("Error: " .. err.kind .. ": " .. err.message)
else
  print(result.content)
  -- result also has: model, finish_reason, usage (sub-table)
end
```

#### ai.complete

Convenience wrapper: sends a single user message and returns just the content string.

```lua
local text, err = ai.complete("Summarize: " .. entity.content)
```

#### ai.embed

Compute vector embeddings for one or more texts. Always returns an array of arrays
(one vector per input), even for a single string input.

```lua
-- Single text
local vecs, err = ai.embed("hello world")
-- vecs[1] is the embedding vector (array of numbers)

-- Batch (one HTTP call for many texts, more efficient)
local vecs, err = ai.embed({"first", "second", "third"})
-- vecs[1], vecs[2], vecs[3] are the embedding vectors

-- Model override
local vecs, err = ai.embed("text", {model = "nomic-embed-text"})
```

**Limits:** Batch input is capped at 2048 texts. Empty strings and empty tables
raise a programming error.

#### Error Handling

AI functions return `(nil, err_table)` for expected runtime failures (network errors,
rate limits, missing config) instead of raising. This is a **deliberate deviation** from
other rela bindings — AI calls are network-bound and scripts should handle failure inline.

Programming errors (wrong argument types, empty messages) still raise via `error()`.

The error table has stable fields for branching:

| Field | Type | Description |
|-------|------|-------------|
| `kind` | string | Error category (see below) |
| `status` | number | HTTP status code (0 for non-HTTP errors) |
| `message` | string | Human-readable description |
| `retry_after` | number | Seconds to wait (for rate limits) |
| `details` | string | Underlying transport error (if any) |

| `err.kind` | When |
|---|---|
| `not_configured` | No `.rela/ai.yaml` or it failed to load |
| `auth` | API key missing/invalid; HTTP 401/403 |
| `bad_request` | HTTP 400/4xx; unknown model |
| `rate_limited` | HTTP 429; check `err.retry_after` |
| `server_error` | HTTP 5xx |
| `timeout` | Request exceeded deadline |
| `network` | DNS, connection refused, TLS |
| `bad_response` | Non-JSON, malformed response |

```lua
local result, err = ai.chat({messages = {{role="user", content="hi"}}})
if err then
  if err.kind == "rate_limited" then
    print("Rate limited, retry after " .. err.retry_after .. "s")
  elseif err.kind == "not_configured" then
    print("AI not configured — create .rela/ai.yaml")
  else
    print("AI error: " .. err.message)
  end
end
```

### Context

| Variable | Description |
|----------|-------------|
| `rela.project_root` | Absolute path to project root |
| `rela.args` | Script arguments (table) |
| `rela.today` | Current date as "YYYY-MM-DD" |

### Markdown AST: Task Lists

The `rela.md` module exposes a markdown AST API for parsing and rendering
content. This section documents the task list (checkbox) support; the rest
of the `rela.md` surface is documented inline in the Go source.

Use `rela.md.parse(content)` to turn a markdown string into an AST table,
and `rela.md.render(ast)` to serialize it back. Task list items (`- [x]`
and `- [ ]`) round-trip through the AST as Lua tables.

#### Task item shape

A task list item is represented as a Lua table with three fields:

```lua
{task = true, checked = <bool>, text = <string>}
```

- `task = true` marks the item as a checkbox item. Only an explicit Lua
  boolean `true` qualifies — strings, numbers, `nil`, and `false` all fall
  through to plain rendering.
- `checked` is the checkbox state (`true` for `[x]`, `false` for `[ ]`).
- `text` is the item's text content.

Plain (non-task) list items are still represented as bare strings, so
existing scripts continue to work without changes.

#### Reading checkbox state

```lua
local ast = rela.md.parse([[
- [x] write the code
- [ ] write the tests
- [x] ~~obsolete item~~
]])

for _, item in ipairs(ast[1].items) do
    if type(item) == "table" and item.task then
        local mark = item.checked and "DONE" or "TODO"
        print(mark, item.text)
    end
end
-- Output:
-- DONE  write the code
-- TODO  write the tests
-- DONE  ~~obsolete item~~
```

#### Building a task list

```lua
local ast = {
    rela.md.list({
        {task = true, checked = true,  text = "design API"},
        {task = true, checked = false, text = "implement"},
        {task = true, checked = false, text = "write docs"},
    }),
}
print(rela.md.render(ast))
-- - [x] design API
-- - [ ] implement
-- - [ ] write docs
```

`rela.md.list(items, ordered?)` accepts plain strings, task tables, or a
mix. Pass `true` as the second argument for an ordered list.

#### Mutating an existing checklist

A common pattern is to read a ticket's checklist, flip a checkbox, and
write it back:

```lua
local ticket = rela.get_entity("TKT-001")
local ast = rela.md.parse(ticket.content)

for _, node in ipairs(ast) do
    if node.type == "list" then
        for _, item in ipairs(node.items) do
            if type(item) == "table" and item.task and item.text == "implement" then
                item.checked = true
            end
        end
    end
end

rela.update_entity("TKT-001", {}, rela.md.render(ast))
```

#### Inline marker preservation policy

When `rela.md.parse` extracts the text of a list item (or paragraph,
heading, blockquote, or table cell), the following inline markers are
**preserved**:

- **Strikethrough** (`~~...~~`) — load-bearing because the checklist
  validation layer treats strikethrough as the "skip" marker for items
  that are intentionally not done.
- **Code spans** (`` `...` ``) — preserved because identifiers and code
  references are structural and round-tripping should not mangle them.

The following inline markers are **dropped** (only the inner text is
captured):

- Bold (`**...**`), italic (`*...*` or `_..._`)
- Links (`[text](url)` becomes just `text`)
- Autolinks and raw HTML

This policy is intentional and pinned by tests in
`internal/lua/markdown_test.go` (`TestMdInlineTextPolicy`). If you need
to round-trip content with full inline fidelity, do not pass it through
`rela.md.parse`/`render` — read and write the raw `entity.content`
string instead.

#### Limitations

- **First text block only.** A list item that contains multiple
  paragraphs, nested lists, or fenced code blocks will only have its
  first text block captured in `text`. This matches the GFM task list
  spec, which requires the checkbox in the first text block.
- **Mixed lists.** A list mixing task and plain items may not be
  symmetrically re-parseable: goldmark only classifies an item as a
  task if it carries its own checkbox. The renderer always emits
  checkbox syntax for items with `task = true`.
- **Sparse item tables.** Scripts that delete items via
  `items[i] = nil` will get a compact rendering (the renderer skips
  `nil` holes), but they should compact the table explicitly if they
  rely on `#items` afterwards — Lua's length operator returns a
  "border", not a count.

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

### AI / Network Access

When `.rela/ai.yaml` is configured, scripts can make HTTP requests to the configured
AI provider via `ai.chat`, `ai.complete`, and `ai.embed`. This means scripts can send
entity content to an external service. **Treat Lua scripts as trusted code** — don't
run scripts from untrusted sources.

API keys are never logged or included in error messages.

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
