---
id: PLAN-BS9IJ
type: planning-checklist
title: 'Planning: Add built-in scheduled task runner for Lua scripts'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- New `rela scheduler` CLI command that runs a long-lived process executing Lua scripts on cron-like schedules
- Schedule configuration in a project-level `schedules.yaml` file
- Lua scripts executed with the same capabilities as `rela script` (entity CRUD, graph queries, AI)
- Graceful shutdown on SIGINT/SIGTERM via context cancellation
- Structured logging of task execution
- Overlap prevention (skip execution if previous run still active)
- Missed run detection: on startup, check last-run timestamps and execute immediately if a scheduled window was missed
- State file (`.rela/scheduler-state.json`) persists last successful run time per task

Out of scope:
- Distributed scheduling / leader election
- Web UI for schedule management
- Schedule-triggered automations (different from the automation engine)
- Retry logic for failed tasks (can be added later)

**Acceptance Criteria:**

1. `rela scheduler` starts and runs Lua scripts according to schedules defined in `schedules.yaml`
2. Schedule configuration supports cron expressions (5-field: min hour dom month dow)
3. Each task references a Lua script by path relative to `scripts/` directory
4. Tasks have access to entity CRUD, graph queries, and AI via standard Lua runtime
5. SIGINT/SIGTERM triggers graceful shutdown: waits for running tasks then exits
6. Each task execution logs start, completion (with duration), and errors
7. If a task is still running when its next schedule fires, the next run is skipped with a warning log
8. Invalid cron expressions or missing scripts cause clear error messages at startup
9. On startup, if a task's scheduled window was missed (scheduler wasn't running), execute it immediately
10. Last-run timestamps are persisted to `.rela/scheduler-state.json` and survive restarts

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Go cron libraries:** `github.com/robfig/cron/v3` is the standard, mature, well-tested cron scheduler for Go. Supports 5-field cron expressions, timezone, per-job wrappers (for overlap prevention, logging).
- **Existing patterns in codebase:**
  - `internal/cli/root.go`: Signal-aware context via `signal.NotifyContext` — scheduler inherits this pattern
  - `internal/script/executor.go`: `script.Engine.ExecuteFile()` handles Lua script execution with security validation (local paths, .lua extension, traversal protection via `os.OpenRoot`)
  - `internal/lua/runtime.go`: `lua.New()` with options like `WithContext()`, `WithAIProvider()`, `WithTimeout()`
  - `internal/cli/mcp.go`: Long-running command pattern with workspace watcher and cleanup via defer
  - `internal/workspace/`: `workspace.Discover()` + `workspace.New()` for project initialization outside PersistentPreRunE
- **No existing scheduled task infrastructure** in rela

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **New package `internal/scheduler/`** with:
   - `Config` struct: loaded from `schedules.yaml`, defines list of `TaskConfig` (name, script path, cron expression, optional timeout override)
   - `Scheduler` struct: wraps `robfig/cron/v3`, manages task lifecycle
   - `Run(ctx)` method: starts cron, blocks until context cancelled, then drains running tasks

2. **New CLI command `rela scheduler`** in `internal/cli/scheduler.go`:
   - Skips PersistentPreRunE metamodel loading (like `mcp`)
   - Uses `workspace.Discover()` + `workspace.New()` for independent initialization (like `mcp`)
   - Loads `schedules.yaml` from project root
   - Creates `scheduler.New(config, workspace)`, calls `scheduler.Run(ctx)`

3. **Task execution** reuses `script.Engine.ExecuteFile()`:
   - Call `workspace.Sync()` before each execution to ensure fresh graph state
   - Each cron fire creates a new `lua.Runtime` via the engine (fresh VM per execution)
   - Context with per-task timeout (from config or default)
   - `robfig/cron/v3` `SkipIfStillRunning` wrapper for overlap prevention (not `DelayIfStillRunning` — skip, don't queue)

4. **Configuration format** (`schedules.yaml`):
   ```yaml
   tasks:
     - name: daily-report
       script: reports/daily.lua
       schedule: "0 9 * * *"
       timeout: 5m
     - name: validate-orphans
       script: checks/orphans.lua
       schedule: "*/30 * * * *"
   ```

5. **Missed run detection** on startup:
   - Load `.rela/scheduler-state.json` (map of task name → last successful run timestamp)
   - For each task, use `robfig/cron`'s parser to compute the previous scheduled time from now
   - If the previous scheduled time is after the last recorded run, execute immediately
   - Update state file after each successful execution (both catch-up and regular runs) using atomic writes via `internal/storage/safefs.go` pattern

6. **Graceful shutdown**: Context cancellation propagates to running Lua VMs via `lua.WithContext()`. Scheduler waits for in-flight tasks (with a hard timeout) before returning.

**Alternatives considered:**
- Embed schedules in `metamodel.yaml`: Rejected — metamodel is for entity/relation schema, mixing concerns. Separate file is cleaner.
- Use stdlib `time.Ticker` instead of cron library: Rejected — cron expressions are complex to parse correctly, `robfig/cron` is battle-tested.
- In-process scheduler within `rela mcp`: Rejected — MCP server has a different lifecycle and concern. Separate command is more composable.

**Files to modify:**
- `internal/scheduler/config.go` (new) — Config types and YAML loading
- `internal/scheduler/state.go` (new) — State persistence (last-run timestamps)
- `internal/scheduler/scheduler.go` (new) — Core scheduler logic with missed-run detection
- `internal/scheduler/scheduler_test.go` (new) — Unit tests
- `internal/cli/scheduler.go` (new) — CLI command
- `internal/cli/root.go` — Add scheduler to skip list, register command
- `go.mod` / `go.sum` — Add `robfig/cron/v3` dependency

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `schedules.yaml` (trusted, project-level config): validate cron syntax at startup, validate script paths exist and are within `scripts/` directory
- Script paths: reuse `script.Engine`'s existing security validation (local paths only, .lua extension, traversal-resistant via `os.OpenRoot`)

**Security-Sensitive Operations:**
- Lua script execution: sandboxed runtime (no io/os/debug libs), same as existing `rela script`
- File access: restricted to `rela.write_file()` output directory
- AI access: same opt-in model as existing scripts (requires `.rela/ai.yaml`)

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. Config loading: valid YAML → correct Config struct
2. Scheduler starts, fires task at correct time (use short intervals in tests)
3. Task has access to rela bindings (entity CRUD, graph)
4. Graceful shutdown: cancel context → scheduler returns after running task completes
5. Overlap prevention: slow task → next scheduled fire is skipped
6. Logging: verify start/complete/error log messages via slog handler
7. Missed run: set last-run to yesterday, start scheduler → task executes immediately
8. No missed run: set last-run to recent → no immediate execution
9. State persistence: run a task → state file updated with timestamp
10. First run (no state file): all tasks treated as missed → execute immediately

**Edge Cases:**
- Empty schedules.yaml (no tasks) → scheduler starts but does nothing
- Script file missing at startup → error before scheduler starts
- Script fails at runtime → logged, scheduler continues
- All tasks overlapping → all skipped except running ones
- Context cancelled while task is running → task gets cancelled via lua.WithContext
- State file corrupted/invalid JSON → treat as no state (all tasks missed)
- State file has entries for tasks no longer in config → ignored (stale entries cleaned up)

**Negative Tests:**
- Invalid cron expression → clear error at startup
- Script path outside scripts/ directory → rejected
- Non-.lua file → rejected
- Missing schedules.yaml → clear error message

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- **Cron library compatibility**: `robfig/cron/v3` is well-maintained but adds a dependency. Mitigation: it's widely used in Go ecosystem, minimal transitive deps.
- **Long-running process stability**: Memory leaks from repeated Lua VM creation. Mitigation: each execution creates and tears down a fresh VM; Go's GC handles cleanup.
- **Graph staleness**: Scheduler reuses workspace across runs; file changes between runs won't be reflected. Mitigation: call `workspace.Sync()` before each task execution to reload from disk.
- **Duplicate task names**: Config keys state by task name; duplicates would silently collide. Mitigation: validate uniqueness at config load time.

**Effort:** L (new package, new CLI command, new config file, cron dependency,
comprehensive tests)

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] CLI help text (new `rela scheduler` command)
- [x] CLAUDE.md (new package, new config file pattern)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (has user-facing docs — see above)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-POYCA (SkipIfStillRunning fix), RR-2WA7R (atomic writes), RR-BU8BK (workspace.Sync), RR-44NJG (duplicate name validation) — all addressed
