---
id: PLAN-F1FH
type: planning-checklist
title: 'Planning: Implement coroutine-based interactive Lua flows'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope (V1):**
- Coroutine-based execution model: Lua scripts yield forms, receive events on resume
- Single screen type: `form` with fields and actions
- 6 field types: `text`, `select`, `multi-select`, `boolean`, `number`, `date`
- CLI transport only (charmbracelet/huh)

**Out of scope (future tickets):**
- Data-entry (web) transport
- MCP transport
- `entity` field type (searchable picker with inline create)
- File upload fields
- Wizard stdlib helpers

**Acceptance Criteria:**

1. **AC1: Form with all field types** - Script emits form with text, select, multi-select, boolean, number, date fields; receives all values on submit
   - Test: Form with each field type, verify data received matches input

2. **AC2: Multi-step flow** - Script emits multiple forms in sequence, handles back/cancel actions
   - Test: 2-step flow, user goes back from step 2, changes step 1 value, continues - final data correct

3. **AC3: CLI transport** - Script runs via `rela flow scripts/example.lua`
   - Test: Run flow in CLI, complete form, verify entity created

4. **AC4: Cancel handling** - User can cancel at any point, script exits cleanly
   - Test: Cancel mid-flow, verify no entities created

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

**Libraries considered:**
- gopher-lua coroutines - **CHOSEN**: Already loaded in Lua runtime, native yield/resume semantics
- Lua channels - Rejected: Coroutines provide simpler semantics for sequential form flows. Channels add complexity (deadlock handling, resource management) without benefit for V1's single-form, single-transport scope. Can be reconsidered if async patterns (timeouts, concurrent inputs) are needed later.
- External wizard libraries - Rejected: Would add dependency, coroutines are simpler

**Similar patterns in codebase:**
- `internal/lua/runtime.go:106` - Coroutine library already loaded
- `internal/dataentry/handlers.go` - Form handling patterns, HTMX integration
- `internal/dataentry/app.go` - State management, session patterns
- `internal/mcp/tools_lua.go` - Existing MCP Lua integration

**External research (script-driven UI systems):**

| System | Pattern | Relevance |
|--------|---------|-----------|
| [Game dialogue systems](https://exelo.tl/lua-coroutines.html) | Lua coroutine yield/resume | **Exact pattern** - `say()` yields, user input resumes |
| [Unity coroutines](https://docs.unity3d.com/Manual/Coroutines.html) | `yield return WaitUntil(condition)` | Same concept, proven in production |
| [Ink](https://www.inklestudios.com/ink/) (Inkle Studios) | Narrative scripting, separate from UI | Script/transport separation model |
| [Yeoman](https://yeoman.io/)/[Plop.js](https://plopjs.com/) | Inquirer.js prompts + actions | CLI prompt patterns |
| [Dialogflow](https://dialogflow.com/docs/concepts/slot-filling) / [Rasa](https://legacy-docs-oss.rasa.com/docs/rasa/forms/) | Slot-filling dialog management | N/A - scripting handles this naturally |
| [XState wizard forms](https://thesilverhand.blog/articles/xstate-wizard-forms/) | FSM-driven forms | Alternative (rejected - coroutines simpler) |
| [charmbracelet/huh](https://github.com/charmbracelet/huh) | Go forms library | **Use for CLI transport** |

**Key insights from research:**

1. **Coroutine pattern is proven** - Game engines have used this exact pattern for decades for cutscenes and dialogue
2. **Script/renderer separation** - Ink's design philosophy: "slot into your own game and UI with ease" - exactly our transport abstraction
3. **Slot-filling is just code** - Unlike Dialogflow/Rasa which need declarative slot definitions, our scripted approach handles dynamic fields naturally:
   ```lua
   -- Skip fields that already have values
   if not answers.priority then
     answers.priority = emit({type = "select", ...}).data.priority
   end
   ```
4. **Use charmbracelet/huh for CLI** - Modern, maintained, accessible, built on Bubble Tea

**Prior art in rela:**
- `lua-scripting` concept documents the existing Lua runtime
- `FEAT-i5ji` is the parent feature for Lua scripting

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Architecture Layers

```
┌─────────────────────────────────────────────────────────┐
│                     Lua Script                          │
│  emit(form) ──yield──▶ ... ◀──resume── event            │
└─────────────────────────────────────────────────────────┘
                    │              ▲
                    ▼              │
            ┌───────────────────────────────┐
            │      Flow Runtime             │
            │  - Manages Lua coroutine      │
            │  - Validates form specs       │
            └───────────────────────────────┘
                    │              ▲
                    ▼              │
               ┌────────┐
               │  CLI   │
               │ (huh)  │
               └────────┘
```

### 1. Lua API

```lua
-- Primitive: yield form, receive event
local event = rela.flow.emit(form)
```

### 2. Form Spec

```lua
form = {
  type = "form",
  title = "Create Ticket",
  description = "Optional explanation",

  fields = {
    {name = "title", type = "text", label = "Title", required = true},
    {name = "desc", type = "text", label = "Description", lines = 5},
    {name = "priority", type = "select", label = "Priority",
     options = {{"high", "High"}, {"medium", "Medium"}, {"low", "Low"}}},
    {name = "tags", type = "multi-select", label = "Tags", options = {...}},
    {name = "public", type = "boolean", label = "Make public"},
    {name = "count", type = "number", label = "Count", min = 0, max = 100},
    {name = "due", type = "date", label = "Due date"},
  },

  actions = {
    {"submit", "Create"},           -- {id, label}
    {"cancel", "Cancel", "warning"}, -- {id, label, style}
  },
}
```

### 3. Field Types (V1)

| Type | Options | Returns | huh Widget |
|------|---------|---------|------------|
| `text` | `required`, `default`, `placeholder`, `lines` | `string` | Input (or Text if lines>1) |
| `select` | `options`, `required`, `default` | selected value | Select |
| `multi-select` | `options`, `required`, `default` | `{value, ...}` | MultiSelect |
| `boolean` | `default` | `true`/`false` | Confirm |
| `number` | `required`, `default`, `min`, `max`, `step` | `number` | Input + validation |
| `date` | `required`, `default`, `min`, `max` | `"YYYY-MM-DD"` | Input + validation |

**Implementation notes for number/date:**

The charmbracelet/huh library doesn't have native number or date widgets. Implementation:

- **number**: Use `huh.Input` with custom validation function
  - Parse as float64, return as Lua number
  - Enforce min/max bounds in validator
  - Empty non-required field → `nil`
  - Reject NaN, Infinity, non-numeric input
  - Use "." as decimal separator (standard, not locale-specific)

- **date**: Use `huh.Input` with custom validation function
  - Accept ISO format: YYYY-MM-DD
  - Validate date is real (reject 2024-02-30)
  - Enforce min/max via string comparison (works for ISO format)
  - Empty non-required field → `nil`
  - No time component (date only)

### 4. Event

```lua
event = {
  action = "submit",  -- matches action id
  data = {
    title = "My ticket",
    priority = "high",
    tags = {"bug", "urgent"},
    public = true,
    count = 5,
    due = "2026-04-15",
  },
}
```

### 5. Error Handling

**Error contract:** `emit()` raises a Lua error on validation/transport failures. Errors propagate to the CLI which displays them and exits.

```lua
-- Errors propagate naturally - CLI shows error message and exits
local event = rela.flow.emit(form)

-- Scripts handle expected conditions via actions, not errors
if event.action == "cancel" then
  return  -- User chose to cancel - not an error
end
```

**Error types:**

| Error | Cause | Behavior |
|-------|-------|----------|
| `validation: missing required field 'title'` | Form spec invalid | Raised immediately, before transport |
| `validation: unknown field type 'foo'` | Invalid field type | Raised immediately |
| `validation: unknown screen type 'bar'` | Invalid screen type | Raised immediately |
| `transport: user interrupted` | User pressed Ctrl+C | Raised on resume (not same as cancel action) |
| `transport: terminal not interactive` | No TTY for CLI | Raised on present |

**Rationale:** Raising errors is idiomatic Lua and matches existing rela bindings. User-initiated cancellation is handled via actions (cancel button), not errors. Transport interruption (Ctrl+C) is an error because it bypasses the script's control flow.

**Note:** `pcall` cannot wrap `emit()` due to gopher-lua limitation ([issue #306](https://github.com/yuin/gopher-lua/issues/306)) - yielding inside pcall throws an error. This is acceptable because:
1. Validation errors indicate script bugs (fix the script)
2. Transport errors are unrecoverable (terminal issues)
3. User cancellation is handled via cancel action, not errors

### 6. Form Spec Validation Schema

**Required fields by screen type:**

| Screen Type | Required | Optional |
|-------------|----------|----------|
| `form` | `type`, `fields`, `actions` | `title`, `description` |

**Field validation:**

| Field Property | Type | Required | Constraints |
|----------------|------|----------|-------------|
| `name` | string | yes | Identifier format: `[a-zA-Z][a-zA-Z0-9_]*`, max 64 chars, unique within form |
| `type` | string | yes | One of: `text`, `select`, `multi-select`, `boolean`, `number`, `date` |
| `label` | string | no | Defaults to titlecased `name` |
| `required` | boolean | no | Defaults to `false` |
| `default` | varies | no | Must match field type |
| `options` | array | yes for select/multi-select | See options validation below |
| `placeholder` | string | no | Only for `text` |
| `lines` | number | no | Only for `text`, enables textarea, must be ≥1 |
| `min`, `max` | number/string | no | For `number`: numeric bounds. For `date`: ISO date strings |
| `step` | number | no | Only for `number`, must be >0 |

**Options validation (for select/multi-select):**

| Constraint | Rule |
|------------|------|
| Format | Array of `{value, label}` tuples (2-element arrays) |
| Non-empty | Must have at least 1 option |
| Max count | Maximum 1000 options |
| Unique values | Option values must be unique within field |
| Value format | Non-empty string, no null bytes |
| Label format | Non-empty string |

**Action validation:**

| Property | Type | Required | Constraints |
|----------|------|----------|-------------|
| `[1]` (id) | string | yes | Non-empty, unique within form |
| `[2]` (label) | string | yes | Non-empty |
| `[3]` (style) | string | no | One of: `primary`, `warning`, `danger` |

### 7. Multi-step Example

```lua
-- Collect data across steps
local data = {}

-- Step 1: Basic Info
::step1::
local e1 = rela.flow.emit({
  type = "form",
  title = "Basic Info",
  fields = {
    {name = "title", type = "text", required = true, default = data.title},
    {name = "kind", type = "select", default = data.kind,
     options = {{"bug", "Bug"}, {"feature", "Feature"}}},
  },
  actions = {{"next", "Next"}, {"cancel", "Cancel"}},
})
if e1.action == "cancel" then return end
data.title = e1.data.title
data.kind = e1.data.kind

-- Step 2: Details
::step2::
local e2 = rela.flow.emit({
  type = "form",
  title = "Details",
  fields = {
    {name = "description", type = "text", lines = 5, default = data.description},
    {name = "priority", type = "select", default = data.priority,
     options = {{"high", "High"}, {"medium", "Medium"}, {"low", "Low"}}},
  },
  actions = {{"back", "Back"}, {"submit", "Create"}, {"cancel", "Cancel"}},
})
if e2.action == "cancel" then return end
if e2.action == "back" then goto step1 end
data.description = e2.data.description
data.priority = e2.data.priority

-- Create entity with collected data
rela.create_entity("ticket", data)
```

**Alternative: Loop-based multi-step** (for wizards with many steps):

```lua
local steps = {}

function steps.basic_info(data)
  local e = rela.flow.emit({
    type = "form",
    title = "Basic Info",
    fields = {
      {name = "title", type = "text", required = true, default = data.title},
      {name = "kind", type = "select", default = data.kind,
       options = {{"bug", "Bug"}, {"feature", "Feature"}}},
    },
    actions = {{"next", "Next"}, {"cancel", "Cancel"}},
  })
  if e.action == "cancel" then return nil end
  data.title = e.data.title
  data.kind = e.data.kind
  return "details"  -- next step name
end

function steps.details(data)
  local e = rela.flow.emit({
    type = "form",
    title = "Details",
    fields = {
      {name = "description", type = "text", lines = 5, default = data.description},
      {name = "priority", type = "select", default = data.priority,
       options = {{"high", "High"}, {"medium", "Medium"}, {"low", "Low"}}},
    },
    actions = {{"back", "Back"}, {"submit", "Create"}, {"cancel", "Cancel"}},
  })
  if e.action == "cancel" then return nil end
  if e.action == "back" then return "basic_info" end
  data.description = e.data.description
  data.priority = e.data.priority
  return nil  -- done
end

-- Run the wizard
local data = {}
local step = "basic_info"
while step do
  step = steps[step](data)
end

-- Create if completed (not cancelled)
if data.title then
  rela.create_entity("ticket", data)
end
```

### 3. Flow Runtime (Go)

```go
// internal/lua/flow.go
type FlowRuntime struct {
    L         *lua.LState
    co        *lua.LState  // The script's coroutine thread
    transport Transport
}

type Transport interface {
    // Present screen to user, block until event received
    Present(screen Screen) (Event, error)
}

type Screen struct {
    Type        string        `json:"type"`
    Title       string        `json:"title,omitempty"`
    Description string        `json:"description,omitempty"`
    Fields      []ScreenField `json:"fields,omitempty"`
    Actions     []Action      `json:"actions,omitempty"`
}

type Event struct {
    Action string                 `json:"action"`
    Data   map[string]interface{} `json:"data,omitempty"`
}
```

**Coroutine lifecycle:**

```
┌─────────────────────────────────────────────────────────────────┐
│  rela flow script.lua                                           │
│                                                                 │
│  1. Load script into LState                                     │
│  2. Create coroutine: co, _ := L.NewThread()                    │
│  3. Get main function: fn := L.GetGlobal("main") or chunk       │
│                                                                 │
│  ┌─────────────── Resume Loop ───────────────┐                  │
│  │                                           │                  │
│  │  4. L.Resume(co, fn, args...)             │                  │
│  │         │                                 │                  │
│  │         ▼                                 │                  │
│  │  ┌─────────────────────────────────┐      │                  │
│  │  │ ResumeOK     → Script finished  │──────┼──► Done          │
│  │  │ ResumeError  → Script errored   │──────┼──► Error         │
│  │  │ ResumeYield  → Script yielded   │      │                  │
│  │  └─────────────────────────────────┘      │                  │
│  │         │                                 │                  │
│  │         ▼                                 │                  │
│  │  5. Extract yielded value (Screen)        │                  │
│  │  6. Validate screen spec                  │                  │
│  │  7. transport.Present(screen) → event     │                  │
│  │  8. Push event table onto stack           │                  │
│  │  9. L.Resume(co, fn) ─────────────────────┘                  │
│  │                                                              │
│  └──────────────────────────────────────────────────────────────┘
└─────────────────────────────────────────────────────────────────┘
```

**Key implementation details:**

1. **Script wrapping**: The script file is loaded and executed inside the coroutine, not wrapped in a function. `emit()` yields from wherever it's called.

2. **emit() implementation**: Registered as `rela.flow.emit`, it:
   - Validates the screen spec table
   - Converts Lua table to Go Screen struct
   - Calls `coroutine.yield(screen)` to suspend
   - On resume, receives event table as return value

3. **Cleanup**: `L.Close()` cleans up both main state and coroutine thread.

4. **Nested coroutines**: If script creates its own coroutines, they work normally. Only `emit()` interacts with the flow runtime's coroutine.

### 4. CLI Transport

**CLI Transport** (`internal/cli/flow.go`):
- Uses [charmbracelet/huh](https://github.com/charmbracelet/huh) for interactive forms
- Built on Bubble Tea (Elm architecture), actively maintained
- Accessible mode for screen readers (`form.WithAccessible(true)`)
- Maps form spec to huh fields: Input, Text, Select, MultiSelect, Confirm
- Synchronous: blocks until user submits/cancels

**Alternatives considered:**

1. **Declarative form spec (rejected):** Less flexible for dynamic branching, script can't adapt based on previous answers. Systems like Dialogflow/Rasa need explicit slot definitions; our scripted approach handles this naturally.

2. **State machine / XState-style (rejected):** Requires explicit state definitions and transition rules. Coroutines give the same power with linear, readable code. As the research shows: "complex forms rarely resemble those state machine drawings, even though they often look exactly like state machines."

3. **Callback-based (rejected):** More complex than coroutines, harder to reason about flow. The game dialogue research explicitly shows coroutines replacing "code that grows unwieldy very quickly" with callbacks.

4. **WebSocket transport (deferred):** HTTP request/response sufficient for now, WebSocket can be added later for real-time updates.

**Dependencies:**

- gopher-lua (existing) - Lua runtime with coroutine support
- charmbracelet/huh (new) - CLI forms library, built on Bubble Tea
- No new external dependencies for HTTP transport

**Design best practices (from research):**

1. **Script isolation** - Run scripts in sandboxed environment (existing Lua sandbox)
2. **Graceful cancellation** - Always allow cancel action; script handles cleanly
3. **No side effects until commit** - Collect all data first, create entities at end
4. **Form as atomic unit** - Transport renders all fields, submits all values together
5. **Transport decides presentation** - CLI may ask sequentially, web shows all at once
6. **Accessible fallback** - CLI uses huh's accessible mode

**Package Structure:**

The flow runtime and transport interface live in `internal/lua/` (not a separate `internal/flow/` package) because:
1. Flow is tightly coupled to Lua coroutine management
2. Follows existing pattern: `internal/lua/runtime.go`, `internal/lua/markdown.go`
3. Avoids import cycle: CLI imports `internal/lua`, not vice versa
4. Transport interface is defined in `internal/lua/flow.go`, implementations in their respective packages

```
internal/lua/
├── runtime.go       # Existing - register rela.flow module
├── flow.go          # NEW - FlowRuntime, Transport interface, Screen/Event types, validation
├── flow_test.go     # NEW - Unit tests with mock transport
└── markdown.go      # Existing

internal/cli/
├── root.go          # Existing - add flow command
└── flow.go          # NEW - CLI transport (huh), `rela flow` command
```

**Files to modify:**

New files:
- `internal/lua/flow.go` - Flow runtime, Transport interface, form/event types, validation, `rela.flow.emit()`
- `internal/lua/flow_test.go` - Unit tests with mock transport
- `internal/cli/flow.go` - CLI transport implementation + `rela flow` command

Modify:
- `internal/lua/runtime.go` - Register `rela.flow` module in `registerBindings()`
- `internal/cli/root.go` - Add flow command to root

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

1. **Screen specs from Lua** - Source: Script yields
   - Validate screen type is known enum
   - Validate required fields present per screen type
   - Reject unknown properties (allowlist)
   - Invalid: Return error to script, abort flow

2. **Events from user** - Source: CLI input, HTTP POST, MCP tool call
   - Validate action is expected for current screen
   - Validate data types match expected fields
   - Sanitize string values (no null bytes)
   - Invalid: Return error response, don't resume coroutine

3. **Flow scripts** - Source: `scripts/` directory
   - Same sandboxing as existing Lua scripts
   - No file system access beyond rela APIs
   - Scripts validated at load time

**Security-Sensitive Operations:**

1. **Entity creation** - Protected by existing workspace validation
2. **Script execution** - Existing sandbox applies (no os/io/debug)

Note: Session storage is not applicable for V1 (CLI transport is synchronous, no sessions).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test Scenario | Approach |
|----|---------------|----------|
| AC1 | All field types | Unit test: mock transport, verify each field type returns correct data type |
| AC2 | Multi-step + back | Unit test: simulate submit/back sequence, verify data preserved |
| AC3 | CLI transport | Integration: run `rela flow` command, verify form renders and entity created |
| AC4 | Cancel handling | Unit test: simulate cancel, verify script exits cleanly |

**Edge Cases:**

- Empty form submission (required fields)
- User cancels mid-flow (cleanup state)
- Script errors during flow (graceful error)
- Invalid screen type from script (validation error)
- Missing required screen fields (validation error)
- Unicode in form fields (preserve correctly)
- Very long text in fields (reasonable limits)
- Rapid back/forward navigation (state consistency)
- User presses Ctrl+C during form input (transport: user cancelled error)

**Negative Tests:**

- Invalid event action → Error response, re-present form
- Invalid data types in event → Validation error
- Script yields non-table → Runtime error
- Malformed screen spec → Validation error before transport
- Script finishes without yielding → Normal completion (no forms shown)

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Coroutine state management complexity | Medium | Medium | Start with simple single-step flows, add complexity gradually. Study gopher-lua coroutine examples. |
| huh library learning curve | Low | Low | Well-documented, simple API, examples in repo |
| Field type edge cases | Medium | Low | Comprehensive test coverage for each type |

**Effort:** M (Medium)

Rationale: While the scope is intentionally minimal (CLI only, single form type), there are several non-trivial pieces:
- Coroutine lifecycle management (create, resume, handle completion/error)
- Form spec validation with detailed error messages
- Field type mapping to huh widgets (6 types × edge cases)
- Error handling design (transport errors, validation errors, script errors)
- Test coverage for all paths

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: V1 is CLI-only, docs will be written when web transport added)

**Documentation Impact:**

- [x] User guide / reference docs - New page on interactive flows
- [x] CLI help text (if commands changed) - `rela flow` command help
- [x] CLAUDE.md (if new patterns) - Document flow scripting patterns
- [x] ~~README.md (if project-level changes)~~ (N/A: no project-level changes)
- [x] ~~API docs (if applicable)~~ (N/A: internal feature)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Title | Severity | Status |
|----|-------|----------|--------|
| RR-pcall-yield | pcall + coroutine.yield incompatibility | **significant** | ✓ addressed |
| RR-field-name-validation | Field name validation underspecified | minor | ✓ addressed |
| RR-options-validation | Select options validation gaps | minor | ✓ addressed |
| RR-number-parsing | Number field parsing unspecified | minor | ✓ addressed |
| RR-date-format | Date field format details missing | minor | ✓ addressed |
| RR-coroutine-lifecycle | Coroutine lifecycle not fully specified | minor | ✓ addressed |
| RR-huh-field-mapping | Verify huh supports all field types | nit | ✓ addressed |

**Resolutions:**

- **RR-pcall-yield**: Removed pcall example. Documented that pcall cannot wrap emit() due to gopher-lua limitation, and why this is acceptable.
- **RR-field-name-validation**: Added identifier format `[a-zA-Z][a-zA-Z0-9_]*`, max 64 chars.
- **RR-options-validation**: Added options validation table with constraints.
- **RR-number-parsing**: Added implementation notes for number field handling.
- **RR-date-format**: Added implementation notes for date field handling.
- **RR-coroutine-lifecycle**: Added lifecycle diagram and implementation details.
- **RR-huh-field-mapping**: Added huh Widget column showing field type mapping.
