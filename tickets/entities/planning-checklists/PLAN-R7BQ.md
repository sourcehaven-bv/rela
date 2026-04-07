---
id: PLAN-R7BQ
type: planning-checklist
title: 'Planning: Add server-side actions to data-entry (Lua scripts with redirect/message responses)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- New `actions` section in `data-entry.yaml` defining named operations
- New `action` field on `NavigationEntry` referencing an action by ID
- New `rela.params` Lua binding populated from config params
- Lua scripts in `actions/` directory at project root
- Scripts return a table: `{redirect, message, message_type}`
- New endpoint `POST /api/v1/_action/{id}`
- Sidebar renders action items as buttons (disabled during in-flight)
- Frontend handles redirect + message toast responses
- Script execution reuses `script.Engine` with new action mode
- Correlation ID for error debugging

OUT of scope:
- Parameter interpolation (no `{{today}}` in params)
- Action confirmation prompts
- Action parameters passed at click-time
- Action context (no `rela.entity`)
- Long-running actions / progress streaming
- Permissions / authentication
- Icons for action items (default only)
- Command palette integration (future)

**Acceptance Criteria:**
1. AC1: `actions:` section parses from `data-entry.yaml`, invalid entries rejected at load
2. AC2: Nav entry with `action: today_note` renders as a button in the sidebar
3. AC3: Clicking button POSTs to `/api/v1/_action/today_note` and runs the script
4. AC4: Script can access `rela.params.foo` populated from config
5. AC5: Script `return {redirect = "/x"}` navigates the frontend
6. AC6: Script `return {message = "hi"}` shows a toast
7. AC7: Script error returns 500 with generic message + correlation ID; full error in server logs
8. AC8: Script path is loaded via `os.OpenRoot` — symlink escapes and path traversal are rejected
9. AC9: Action handler takes workspace write lock; concurrent POSTs serialize
10. AC10: Action ID must match `^[a-z0-9_-]{1,64}$`; config rejected otherwise
11. AC11: Script file existence checked at config load; missing file → startup error
12. AC12: Frontend button is disabled while request in flight (prevents double-submit)
13. AC13: `redirect` URL must start with `/` and not `//` (no open redirects)

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Key existing code:**

- `internal/script/executor.go:79-118` — **hardened script loading via `os.OpenRoot` — reuse this**
- `internal/script/executor.go:39-77` — `ExecuteCode`/`ExecuteFile` with `ScriptContext` — extend for actions
- `internal/metamodel/script_context.go` — `ScriptContext` interface (already optional entity)
- `internal/lua/runtime.go:170-179` — `rela.args` population pattern (for `rela.params`)
- `internal/lua/runtime.go:198` — `PCall(0, lua.MultRet, nil)` already captures return values on the stack (currently discarded)
- `internal/dataentry/api_v1.go:693-743` — write handler pattern (clone entity) with `a.mu.Lock()`
- `internal/dataentry/api_v1.go:1540` — `navEntryToSidebarItem` — extend for `Action` field
- `internal/dataentryconfig/config.go:254-268` — `NavigationEntry` — add `Action string`
- `frontend/src/components/common/Sidebar.vue:93-117` — conditional render action as `<button>`
- `frontend/src/stores/ui.ts:74-108` — `uiStore.success/error/warning/info`
- `frontend/src/api/client.ts` — `api.post<T>`

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Key design decisions (post design review):**

1. **Return value via `return` statement, not `rela.respond`** (RR-C9LK). Scripts use idiomatic Lua: `return {redirect = "/x"}`. The runtime already calls `PCall(0, MultRet, nil)` — we read `L.Get(-1)` after successful execution.

2. **Extend `script.Engine` with action mode** (RR-4000, RR-LOFJ). Adds `ExecuteAction(path, ctx, params)` that returns `(*ActionResponse, error)`. Reuses the hardened `loadScript` with `os.OpenRoot`. Path validation is automatic (same as `scripts/` loader).

3. **Take workspace write lock** (RR-Z3R9). Handler acquires `a.mu.Lock()` for the duration of script execution, matching the clone handler pattern.

4. **Short timeout, explicitly set** (RR-CSTD). Pass `lua.WithTimeout(5*time.Second)` when creating the runtime. Document that this is tighter than the CLI default because of the write lock.

5. **Correlation ID** (RR-GUXG). Generate UUID per request, include in error response and server log entry.

### Backend (Go)

**1. Config** (`internal/dataentryconfig/config.go`)

```go
type Action struct {
    Description string            `yaml:"description,omitempty" json:"description,omitempty"`
    Script      string            `yaml:"script" json:"script"`
    Params      map[string]string `yaml:"params,omitempty" json:"params,omitempty"`
}

// Config struct:
Actions map[string]Action `yaml:"actions,omitempty"`

// NavigationEntry struct:
Action string `yaml:"action,omitempty" json:"action,omitempty"`
```

**2. Config validation** (`internal/dataentryconfig/validate.go`)

- Action IDs must match `^[a-z0-9_-]{1,64}$`
- `script` field non-empty, ends in `.lua`, no `..`, no absolute path
- Script file must exist in `{project}/actions/` (verified via `os.OpenRoot(actions).Open(script)`)
- Nav entry `action:` must reference an existing action ID
- `actions/` dir may not exist if `Actions` map is empty; if map is non-empty, dir missing → error

**3. script.Engine extension** (`internal/script/executor.go`)

```go
type ActionResponse struct {
    Redirect    string `json:"redirect,omitempty"`
    Message     string `json:"message,omitempty"`
    MessageType string `json:"message_type,omitempty"`
}

// ExecuteAction runs a Lua script in action mode. The script's return
// value is a table that becomes the ActionResponse. Scripts can also
// use rela.params (map[string]string) and standard rela.* helpers.
func (e *Engine) ExecuteAction(scriptPath string, ctx metamodel.ScriptContext,
    params map[string]string, timeout time.Duration) (*ActionResponse, error)
```

Implementation:
- Use existing `loadScript(projectRoot, path)` which uses `os.OpenRoot` — same path validation as regular scripts
- Create runtime with `lua.WithTimeout(timeout)` and new `lua.WithParams(params)` option
- Call `runtime.RunActionString(code)` — a new method that does `PCall(0, MultRet, nil)` and reads top-of-stack as the return value
- Convert the Lua table to `ActionResponse` struct
- Validate the response:
  - `redirect`: if non-empty, must start with `/` but not `//` (no open redirects)
  - `message_type`: must be empty or one of `success|info|warning|error`
  - Unknown fields in the table → logged warning (not an error)

**4. Lua runtime changes** (`internal/lua/runtime.go`)

```go
type Runtime struct {
    // existing fields
    params map[string]string
}

func WithParams(params map[string]string) Option {
    return func(r *Runtime) { r.params = params }
}

// RunActionFile/RunActionString execute a script and return the top-of-stack
// value (expected to be a table) as a Go interface{}. Also raises an error
// if rela.output is called during action execution (warning only for v1).
func (r *Runtime) RunActionFile(path string) (interface{}, error)
```

In `registerBindings`:
```go
paramsTable := r.L.NewTable()
for k, v := range r.params {
    r.L.SetField(paramsTable, k, lua.LString(v))
}
r.L.SetField(rela, "params", paramsTable)
```

For `rela.output` in action context: add a `r.isAction` flag; when set,
`luaOutput` logs a warning via the stdout writer (no error — don't break
scripts).

**5. HTTP handler** (`internal/dataentry/api_v1.go`)

```go
var actionIDRegex = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

const actionTimeout = 5 * time.Second

func (a *App) handleV1Action(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        w.Header().Set("Allow", "POST")
        writeV1Error(w, r, 405, "method_not_allowed", "Method not allowed", "")
        return
    }

    id := strings.TrimPrefix(r.URL.Path, "/api/v1/_action/")
    if !actionIDRegex.MatchString(id) {
        writeV1Error(w, r, 400, "invalid_action_id", "Invalid action ID", "")
        return
    }

    action, ok := a.Cfg.Actions[id]
    if !ok {
        writeV1Error(w, r, 404, "action_not_found", "Action not found", "")
        return
    }

    // Generate correlation ID for error tracing
    correlationID := generateUUID() // short random string

    // Take write lock (action may mutate workspace)
    a.mu.RUnlock()
    a.mu.Lock()
    defer func() {
        a.mu.Unlock()
        a.mu.RLock()
    }()

    ctx := &actionScriptContext{
        ws:          a.ws,
        meta:        a.meta,
        projectRoot: a.ws.Paths().Root,
    }

    engine := script.NewEngine()
    resp, err := engine.ExecuteAction(action.Script, ctx, action.Params, actionTimeout)
    if err != nil {
        log.Printf("action %s failed [correlation=%s]: %v", id, correlationID, err)
        writeV1JSON(w, 500, map[string]interface{}{
            "error":          "action_failed",
            "message":        "Action failed",
            "correlation_id": correlationID,
        })
        return
    }

    if resp == nil || (resp.Redirect == "" && resp.Message == "") {
        w.WriteHeader(http.StatusNoContent)
        return
    }
    writeV1JSON(w, 200, resp)
}
```

**6. Sidebar handler** (`navEntryToSidebarItem`)

```go
case entry.Action != "":
    item.Action = entry.Action
    // Href stays empty
```

`V1SidebarItem` gets a new `Action string` field (omitempty).

### Frontend (Vue 3)

**1. Types** (`frontend/src/types/config.ts`)

```typescript
export interface SidebarItem {
  label: string
  href?: string // optional now
  icon?: string
  count?: number
  action?: string
}
```

**2. API client** (`frontend/src/api/actions.ts` — new)

```typescript
import { api } from './client'

export interface ActionResponse {
  redirect?: string
  message?: string
  message_type?: 'success' | 'info' | 'warning' | 'error'
}

export async function runAction(id: string): Promise<ActionResponse | null> {
  return api.post<ActionResponse | null>(`/_action/${id}`)
}
```

Exported from `src/api/index.ts`.

**3. Sidebar** (`frontend/src/components/common/Sidebar.vue`)

- Track per-item loading state: `const actionLoading = ref<Set<string>>(new Set())`
- Conditional template: `<RouterLink v-if="item.href">` else `<button v-else-if="item.action" type="button" :disabled="actionLoading.has(item.action)" @click="handleAction(item)">`
- `handleAction`:
  - Add to loading set
  - Call `runAction(item.action)`
  - On success: if `response?.redirect` → `router.push(response.redirect)`; if `response?.message` → `uiStore[message_type || 'success'](response.message)`
  - On error: extract correlation ID from response body, show `uiStore.error("Action failed (ref: xxx)")`
  - Finally: remove from loading set
- Button has `type="button"`, `aria-label="{item.label}"`, visual disabled state

### Files to modify

Backend:
- `internal/dataentryconfig/config.go` — Action struct, Config.Actions, NavigationEntry.Action
- `internal/dataentryconfig/validate.go` — validate actions (ID regex, script exists, nav refs)
- `internal/dataentryconfig/validate_test.go` — tests
- `internal/lua/runtime.go` — `WithParams` option, `rela.params` binding, `RunActionFile` method
- `internal/lua/runtime_test.go` — tests
- `internal/script/executor.go` — `ExecuteAction` method, `ActionResponse` struct
- `internal/script/executor_test.go` — tests
- `internal/dataentry/api_v1.go` — `handleV1Action`, `V1SidebarItem.Action`, route registration
- `internal/dataentry/api_v1_test.go` — tests

Frontend:
- `frontend/src/types/config.ts` — `action?` field on `SidebarItem`
- `frontend/src/api/actions.ts` — new
- `frontend/src/api/index.ts` — export
- `frontend/src/components/common/Sidebar.vue` — button branch + handler with loading state

**Alternatives considered:**

- `rela.respond()` over return value — rejected (RR-C9LK): script return via `MultRet` is already supported by existing `PCall` and is the idiomatic Lua pattern.
- Bypass `script.Engine` — rejected (RR-4000, RR-LOFJ): engine's hardened `os.OpenRoot` loader is exactly what we need; extending it is simpler than rewriting.
- Param interpolation — rejected: user explicitly said no.
- Permissions — rejected: data-entry server assumes trusted local access (same as all other write operations).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Action ID from URL path** — validated against regex `^[a-z0-9_-]{1,64}$`, then allowlist via map lookup. Rejects slashes, dots, null bytes, unicode tricks. Null-byte test explicitly included.
- **Script path from config** — loaded via `os.OpenRoot(projectRoot).OpenRoot("actions").Open(scriptName)` which prevents symlink escapes, TOCTOU, and case-insensitive FS tricks. Script existence validated at config load time.
- **Params from config** — trusted config file, passed as string values only.
- **HTTP method** — only POST accepted.
- **`redirect` from script** — validated server-side: must start with `/` but not `//` (no `//evil.com` open redirects). Frontend also validates defensively.
- **`message_type`** — validated against enum; invalid values rejected.

**Security-Sensitive Operations:**

- File read sandboxed via `os.OpenRoot` (same as existing scripts/).
- Lua runtime sandboxed (no `io`, `os`, `debug`).
- Workspace mutations serialized via `a.mu.Lock()` (no concurrent races).
- Script timeout: 5s (tight because of write lock).
- Error responses: generic message + correlation ID; full error in server logs only.
- No authentication — data-entry server assumes trusted local access.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. **Config** (`validate_test.go`):
   - Valid actions + nav refs load
   - Invalid action ID regex → error
   - Missing script field → error
   - Path traversal in script (`../foo.lua`) → error
   - Script file doesn't exist → error at config load
   - Nav entry referencing undefined action → error
   - `actions/` dir missing but actions declared → error
   - `actions/` dir missing and no actions declared → OK

2. **Lua runtime** (`runtime_test.go`):
   - `WithParams` populates `rela.params` as a table
   - Empty params → empty table
   - `RunActionFile` returns top-of-stack table as Go map
   - Script with no return → nil response
   - Script that errors → error propagated

3. **script.Engine** (`executor_test.go`):
   - `ExecuteAction` with valid script returns response
   - Symlink in actions/ pointing outside → rejected
   - Invalid script path (`..`, absolute) → rejected
   - Script raising Lua error → wrapped error returned
   - Redirect starting with `//` → rejected
   - Invalid `message_type` → rejected

4. **HTTP endpoint** (`api_v1_test.go`):
   - POST existing action → 200 with response or 204 empty
   - POST unknown action → 404
   - POST with invalid ID format → 400
   - POST with null byte in ID → 400
   - GET → 405
   - Script error → 500 with correlation ID
   - **Concurrency**: two parallel POSTs serialize (no duplicates, no corruption)
   - Script returning `redirect` → body contains sanitized redirect

5. **Frontend**: Manual puppeteer verification
   - Action item renders as button
   - Click triggers POST
   - Button disabled during in-flight
   - Redirect navigates
   - Message shows toast
   - Double-click doesn't fire second request

**Edge Cases:**

- Empty params map — `rela.params` is empty table
- Multiple nav entries referencing same action — both work, each click is independent
- Script calls `rela.output` in action mode → warning logged, output dropped
- Script returns non-table → logged, `nil` response
- Empty actions map → no breakage
- Actions dir missing and actions configured → startup error

**Negative Tests:**

- Symlink in `actions/foo.lua` → 400 at load OR 500 at request (os.OpenRoot refuses)
- Script file doesn't exist at config load → startup error
- Script file deleted between load and request → 500 with error in logs
- Script infinite loop → timeout kicks in after 5s, returns 500
- Script panics → recovered, wrapped as error
- Redirect `//evil.com` → rejected with 500 (sanitization failure)
- Redirect `https://evil.com` → rejected
- Unknown `message_type` → rejected

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Path traversal**: mitigated by `os.OpenRoot` (reuses hardened pattern)
- **Workspace corruption**: mitigated by write lock
- **DoS via slow scripts**: 5s timeout + write lock means worst case 5s freeze; acceptable for local personal tool
- **Open redirect**: mitigated by server-side validation of redirect URL
- **Open enum value**: mitigated by server-side validation of message_type
- **Scope creep**: static params only, no interpolation

Effort: **M**

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] User guide / reference docs (data-entry.yaml actions schema + Lua API: `rela.params`, return-value contract)
- [ ] ~~CLI help text~~ (N/A)
- [ ] ~~CLAUDE.md~~ (N/A)
- [ ] ~~README.md~~ (N/A)
- [x] Example action scripts for common patterns (today-note, find-or-create)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Critical (addressed):
- RR-LOFJ — Reuse `os.OpenRoot` via `script.Engine`
- RR-Z3R9 — Handler takes `a.mu.Lock()`
- RR-CSTD — Explicit 5s timeout
- RR-EVXN — Action ID regex `^[a-z0-9_-]{1,64}$`

Significant (addressed):
- RR-C9LK — Use script `return` value via MultRet (no `rela.respond`)
- RR-4000 — Reuse `script.Engine` (new `ExecuteAction` method)
- RR-3LIN — Validate script file exists at config load
- RR-31S8 — Validate `redirect` prefix, enum `message_type`, strict table shape
- RR-UZEX — `rela.output` in action context → warning logged, output dropped
- RR-GUXG — Correlation ID in error responses
- RR-I66V — `type="button"`, `aria-label`, disabled during in-flight
- RR-KC2U — Double-click protection via disabled state

Minor/leverage items deferred to follow-up tickets:
- Command palette integration
- Entity-returning actions with auto-routing (`{created = entity}`)
- Action audit log
- Extract shared `loadScript` helper across 4 call sites
- Icon/affordance for action items
