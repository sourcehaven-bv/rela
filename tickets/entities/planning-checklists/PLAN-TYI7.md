---
id: PLAN-TYI7
type: planning-checklist
title: 'Planning: Web transport for Lua flows in data-entry'
status: pending
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

See TKT-011J body for the full scope. Summary:

**In scope:**
- `lua.HTTPTransport` implementation backed by channels
- In-memory flow session manager with TTL eviction
- HTML templates (server-rendered, HTMX-driven) for all existing field types
- `/flows` page listing discoverable flow scripts
- New `NavigationEntry.Flow` field so users can reference a flow from `data-entry.yaml` navigation
- E2E tests (single-step, multi-step, cancel, expiry)

**Out of scope:**
- MCP transport
- Persisting in-flight flow state across restarts
- New flow field types
- Any change to the Lua API (`rela.flow.emit` stays byte-identical)

**Acceptance Criteria:**

1. **CLI/web parity** — `rela flow foo.lua` and the web launcher run the same script without modification.
*Test:* A golden script `tickets/scripts/qa-multi-step.lua` (already present)
passes in both transports; assert resulting entity state is identical.
2. **`/flows` launcher page** — lists all `.lua` scripts under the project's configured flow directory.
*Test:* E2E test creates 2 flow scripts in a tmp project, hits `/flows`, asserts
both are listed with correct titles.
3. **Menu integration** — `data-entry.yaml` can declare `navigation: [{label: "New ticket", flow: "new_ticket.lua"}]` and the entry renders in the sidebar linking to the flow launcher.
*Test:* Config-driven test: load a config with a flow nav entry, render the
sidebar template, assert the link is present with the correct href.
4. **Multi-step state preservation** — a 3-step flow that collects different fields at each step completes with all values intact in the final entity.
*Test:* E2E test using a multi-step flow; after step 3, a new entity exists with
fields from all three steps.
5. **Clean cancellation** — cancelling a flow mid-step kills the coroutine goroutine, removes the session, and leaves no entities behind.
*Test:* Start a flow that would create an entity on submit, POST the cancel
action at step 2, assert: session gone, no entity, goroutine count returns to
baseline after GC.
6. **Session TTL** — abandoned sessions are evicted after TTL; attempting to resume returns 410 Gone with a friendly message.
*Test:* Start a flow, advance clock past TTL, attempt to POST to the session
URL, assert 410 + message.
7. **E2E coverage** — playwright (or equivalent) test runs the full happy path through the browser.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **`internal/lua/flow.go`** — transport-agnostic `Transport` interface already exists (`Present(Screen) (Event, error)`). `FlowRuntime.run()` is a synchronous for-loop that calls `transport.Present()` at each `coroutine.yield`. This is the key extension point.
- **`internal/cli/flow.go`** — reference `TerminalTransport` using charmbracelet/huh. Same `Screen`→widgets mapping we'll mirror on the server in HTML.
- **`internal/dataentry/actions.go`** — existing pattern for script invocation from data-entry. Does a write-lock upgrade. ⚠ Flows cannot reuse this pattern as-is: they hold state across HTTP requests, so we cannot hold the write lock for the whole flow.
- **`internal/dataentryconfig/config.go:263`** — `NavigationEntry` is already a union type with an `Action` field. Adding `Flow string` follows the existing pattern exactly.
- **`internal/lua/rela-docs/entities/concept/lua-scripting.md`** — documents existing Lua scripting. The flow API is documented in FEAT-YIOF.
- **HTMX pattern**: existing data-entry UI already uses HTMX form posts extensively (see `handleForm` / `handleCreate` in handlers). We'll reuse the same template conventions.

No third-party library needed. The existing `Transport` abstraction is precisely
the seam we need.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Goroutine-per-session model

Each active flow runs on its own goroutine. The `HTTPTransport.Present()` method
blocks on a Go channel; the HTTP handler for form POSTs sends the parsed `Event`
on that channel. The flow goroutine resumes its coroutine, processes the next
yield, and `Present()` sends the next `Screen` back via another channel, where
the GET handler picks it up to render.

Concretely:

```go
// internal/lua/http_transport.go (NEW)
type HTTPTransport struct {
    screens chan Screen    // flow -> handler
    events  chan Event     // handler -> flow
    done    chan struct{}  // closed when flow exits
}

func (t *HTTPTransport) Present(s Screen) (Event, error) {
    select {
    case t.screens <- s:
    case <-t.done:
        return Event{}, ErrFlowCancelled
    }
    select {
    case ev := <-t.events:
        return ev, nil
    case <-t.done:
        return Event{}, ErrFlowCancelled
    }
}
```

### Session manager

```go
// internal/dataentry/flows.go (NEW)
type FlowSession struct {
    ID        string
    Script    string
    Transport *lua.HTTPTransport
    Current   lua.Screen // last yielded screen, for re-render on GET
    CreatedAt time.Time
    LastSeen  time.Time
    resultErr error
}

type FlowSessionManager struct {
    mu       sync.Mutex
    sessions map[string]*FlowSession
    ttl      time.Duration
    ws       *workspace.Workspace
    meta     *metamodel.Metamodel
}
```

On flow start (`POST /flows/{script}/start`):
1. Generate session ID (crypto/rand, 16 bytes hex).
2. Build `lua.Runtime` and `HTTPTransport`.
3. Launch goroutine: `flow.RunFile(...)`. When it returns, close `done` and record `resultErr`.
4. Receive the first screen from `transport.screens`; store in session; redirect to `/flows/session/{id}`.

On step render (`GET /flows/session/{id}`):
1. Look up session.
2. Render the stored `Current` screen as HTML using a template.

On step submit (`POST /flows/session/{id}`):
1. Parse form body into `lua.Event` (field name → value, plus the action).
2. Send on `transport.events`.
3. Block on: next screen from `transport.screens` (advance) OR `done` (flow finished).
4. If advanced: update `Current`, return 303 to GET. If finished: delete session, redirect to `/flows` with flash.

### Concurrency & locking

- The flow goroutine **never** holds the app's write lock across HTTP requests. Instead, it acquires the workspace write lock only for the brief moments when the Lua script calls mutating functions (`rela.create_entity`, etc.). This is already how `internal/lua` works — the runtime takes the lock per-call, not per-run.
- **Verification step during implementation:** audit `internal/lua` to confirm that locks are per-operation, not held across script execution. If they are held for the whole run (as `handleV1Action` does), we need to refactor that first. Flag as a blocker if so.
- The session manager itself uses a `sync.Mutex` around the `sessions` map.

### HTML rendering

- New templates under `internal/dataentry/templates/flows/`:
  - `launcher.html` — lists flows
  - `form.html` — renders a `lua.Screen` with all field types
  - `expired.html` — session expired message
- Each field type maps to an existing widget (reuse the `WidgetText`, `WidgetSelect`, etc. constants from `dataentryconfig`). Avoid duplicating the markup.

### Flow discovery

- Read scripts from `<project>/scripts/flows/` (new conventional path). Fall back to the project's existing scripts dir if configured.
- A flow is any `.lua` file that, when parsed, calls `rela.flow.emit` at least once. For the launcher list, we don't need to parse — just list `.lua` files. The label can be derived from a leading comment `-- @flow: Title here` or fall back to the filename.

### Navigation integration

Add `Flow string` to `NavigationEntry` in `internal/dataentryconfig/config.go`.
Update `validateNavEntry` to accept it as one of the mutually-exclusive item
types. Update the sidebar template/handler to render flow entries as links to
`/flows/{script}/start`.

**Alternatives considered:**

1. **Stateless flows (serialize state to hidden fields)** — Rejected. gopher-lua coroutines cannot be serialized; this would require rewriting flows as FSMs, breaking the core value of FEAT-YIOF (scripts control flow imperatively).
2. **Run flow synchronously per request, replaying from step 0 each time** — Rejected. Would re-run side effects on each step and break scripts that compute intermediate values.
3. **Server-Sent Events for screen delivery** — Rejected as premature. Plain form POSTs with redirects are simpler and sufficient. Can add SSE later if flows need push updates.
4. **New `rela-flow-server` binary** — Rejected. The data-entry server already hosts the web UI; adding a second binary fragments the UX.

**Files to modify:**

- `internal/lua/http_transport.go` (NEW) — channel-based transport
- `internal/lua/http_transport_test.go` (NEW) — unit tests
- `internal/dataentry/flows.go` (NEW) — session manager + HTTP handlers
- `internal/dataentry/flows_test.go` (NEW) — handler tests
- `internal/dataentry/templates/flows/launcher.html` (NEW)
- `internal/dataentry/templates/flows/form.html` (NEW)
- `internal/dataentry/templates/flows/expired.html` (NEW)
- `internal/dataentry/router.go` — register `/flows` routes
- `internal/dataentry/app.go` — initialize `FlowSessionManager`; start TTL cleanup goroutine
- `internal/dataentryconfig/config.go` — add `NavigationEntry.Flow` field
- `internal/dataentryconfig/validate.go` — validate flow nav entries
- `internal/dataentryconfig/validate_test.go` — test cases for flow nav entries
- `internal/dataentry/templates/sidebar.html` (or equivalent) — render flow nav entries
- `internal/dataentry/e2e_test.go` — E2E flow happy path
- `docs/` — user-facing doc for how to configure flows in data-entry (docs-checklist)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

1. **Script path (from URL `/flows/{script}/start`)** — MUST be validated against an allowlist. Approach:
   - Enumerate `.lua` files in the scripts directory once on request.
   - The URL parameter must exactly match one of those filenames (no path separators, no `..`).
   - Reject anything containing `/`, `\`, `..`, null bytes.
   - Use the same `path_security` patterns already present in `internal/dataentry/middleware_security.go`.
2. **Session ID (from URL)** — Must match `^[a-f0-9]{32}$`. Look up in map; reject if absent.
3. **Form values on submit** — Parsed via `net/http` form parser. Values are handed to `lua.Event.Data` which is then validated by the existing `validateEventData` in `internal/lua/flow.go`. Existing validation handles field name allowlist, required fields, types.
4. **Action ID on submit** — Must be one of the actions in the current screen.
5. **`data-entry.yaml` flow references** — validated at config-load time against the actual files in the scripts directory. Invalid references produce a config error.

**Security-Sensitive Operations:**

- **File read (script loading):** Already handled by existing `lua.FlowRuntime.RunFile` which uses absolute paths. We pass a cleaned path from the allowlist, so traversal is impossible.
- **Lua script execution:** The Lua sandbox is already constrained by the existing `lua.Runtime`. Flows do not grant any new capabilities to scripts.
- **Session storage:** In-memory only, so no persistence attack surface. TTL bounds memory usage.
- **CSRF:** The existing data-entry app already has CSRF middleware. The flow submit endpoint will be under the same middleware.
- **Goroutine exhaustion:** An attacker could start thousands of flows to exhaust goroutines. Mitigation: cap the number of concurrent sessions (e.g., 100) and reject new starts when full.
- **Error messages:** Flow errors are logged with a correlation ID; the HTTP response shows a generic "Flow failed" message + correlation ID, matching the `handleV1Action` pattern.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC # | Test location | What it verifies |
|------|---------------|------------------|
| 1 | `internal/dataentry/flows_test.go` | Running `qa-multi-step.lua` through HTTPTransport yields identical Event sequence to TerminalTransport (mocked transport equivalence) |
| 2 | `internal/dataentry/flows_test.go::TestLauncherLists` | GET `/flows` returns HTML containing all `.lua` files from the scripts dir |
| 3 | `internal/dataentryconfig/validate_test.go` + `internal/dataentry/app_test.go` | Config with flow nav entry validates; sidebar rendering includes the entry |
| 4 | `internal/dataentry/flows_test.go::TestMultiStep` | POST start → GET form → POST step1 → GET form → POST step2 → POST step3 → entity created with all three field sets |
| 5 | `internal/dataentry/flows_test.go::TestCancel` | POST cancel action deletes session; no entity created; goroutine count returns to baseline after 200ms GC |
| 6 | `internal/dataentry/flows_test.go::TestSessionExpiry` | Inject a fake clock, advance past TTL, GET session returns 410 with expired template |
| 7 | `internal/dataentry/e2e_test.go::TestFlowHappyPath` | Full browser flow: launcher → start → step1 → step2 → submit → entity exists on disk |

**Edge Cases:**

- **Empty fields dir** — `/flows` renders "No flows available"
- **Script with zero `emit` calls** — flow finishes immediately; redirect to `/flows` with success flash
- **Script that errors mid-flow** — goroutine exits with error; next HTTP interaction returns 500 with correlation ID; session cleaned up
- **User navigates away, comes back before TTL** — GET session re-renders the Current screen; no lost state
- **User double-submits (browser back + resubmit)** — Second submit finds the channel already received; either we drop the duplicate or treat it as idempotent. **Decision:** use a monotonically increasing step counter in the URL so stale submits 409
- **Concurrent flows by same user** — Two different session IDs, each with own goroutine; no interference
- **Script path with unicode/emoji filenames** — supported but must pass allowlist match
- **Server shutdown with active flows** — On shutdown, close all `done` channels so goroutines exit cleanly; do not wait indefinitely
- **Max concurrent sessions exceeded** — Return 503 with retry-after
- **Flow yields a screen with zero actions** — Validation in `parseScreen` already rejects this; extra test ensures HTTP path surfaces it as a 500

**Negative Tests:**

- Malformed session ID → 400
- Unknown session ID → 404
- Script path containing `..` → 400
- Step counter mismatch → 409
- TTL exceeded → 410
- Invalid action ID on submit → 400
- Missing required field → re-render form with error (matches CLI behavior where huh re-prompts)

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **⚠ Workspace locking model is incompatible with long-lived flows** (HIGH)
   - If `internal/lua` holds the workspace write lock for the lifetime of a script run (as `handleV1Action` does via `a.mu.Lock()` before `ExecuteAction`), then a flow goroutine waiting for user input would deadlock the whole server.
   - **Mitigation:** First task of implementation is to audit the locking model. If it's per-run, refactor to per-operation locking in `internal/lua` before doing any HTTP transport work. If that refactor is too large, downgrade this ticket to CLI-only improvements and file a separate ticket for the refactor.
   - **Fallback:** Run flow goroutines entirely outside the app lock; re-acquire the lock only when calling into `workspace` methods. This requires careful review.
2. **Goroutine leak on cancellation** (MEDIUM)
   - gopher-lua coroutines cannot be forcefully killed; a misbehaving script could block on a non-transport operation and never observe the cancel signal.
   - **Mitigation:** HTTPTransport checks `done` on every `Present`. For scripts that hang outside `Present`, rely on the existing per-operation timeout from `lua.Runtime`. Document as a known limitation.
3. **gopher-lua coroutine reentrancy** (LOW)
   - Known bug from TKT-3ILY: `pcall + coroutine.yield` is incompatible in gopher-lua (RR-pcall-yield). This was already addressed there; the HTTPTransport doesn't introduce new reentrancy paths.
4. **Template injection** (LOW)
   - All field values rendered via Go's `html/template`; no raw `template.HTML` for user-controlled content.
5. **Test flakiness from goroutine coordination** (MEDIUM)
   - Channel-based transport with timeouts can flake under load.
   - **Mitigation:** Use synchronous channels with deterministic `chan` ordering; inject a fake clock for TTL tests; use `testing.T.Cleanup` to drain sessions.

**Effort:** **m** — non-trivial because of locking audit + session manager +
templates + E2E, but all extension points already exist and the alternatives
(stateless, serialization) were rejected.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide — new section "Flows in data-entry" covering how to launch flows from the web UI and configure them in navigation
- [x] CLI help text — no changes (CLI flow command is unchanged)
- [x] CLAUDE.md — no changes (no new patterns other than what's explained in existing concepts)
- [x] README.md — mention web flows in the features list if flows are mentioned there
- [ ] API docs — N/A (no public API change)
- [ ] Reference docs for `data-entry.yaml` — document the new `flow:` field on `NavigationEntry`

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- Will be populated after /design-review -->
