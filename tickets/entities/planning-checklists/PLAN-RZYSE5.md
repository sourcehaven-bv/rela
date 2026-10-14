---
id: PLAN-RZYSE5
type: planning-checklist
title: 'Planning: Reload schedules and subscriptions without a restart'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: reloading `schedules.yaml` and `subscriptions.yaml` in a running
scheduler (`rela scheduler`, `rela-server`, desktop); keeping the running config
when an edit is invalid; docs.

Out of scope: file-system watching (polling each tick also works for the sqlite
config layer and for every storage backend); reloading the schema itself (a
schema change already rebuilds the server generation); cancelling a run that is
in flight.

**Acceptance Criteria:**
1. A subscription added to a running scheduler is polled within one tick. Test: scheduler unit test with a mock workspace whose file changes between ticks, and an appbuild e2e test that writes `subscriptions.yaml` after start.
2. A removed task gets no new runs; a queued run still finishes. Test: unit test (no job enqueued after removal; the queued subscription job fails as "not configured" and is recorded; a queued script job runs from its payload).
3. An invalid edit keeps the previous tasks and logs once per distinct error. Test: unit test with a capturing logger across three ticks.
4. A scheduler started with no tasks picks up a file created later. Test: StartBackground test.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: reuses the scheduler's own loader)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** dataentry reloads `data-entry.yaml` and keeps the
previous config on error (internal/dataentry/watcher.go reloadConfig);
realtime.yaml does the same (appbuild/realtime.go Reload). CLAUDE.md requires
`atomic.Pointer` for published state and one load per operation.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `Scheduler.config` becomes `atomic.Pointer[Config]`; `tick`, `taskNamed`, `task` and `Run` load it once. `pruneAge` is computed from the loaded config at prune time.
- `WatchConfig()` sets `s.reload = func(ctx) (*Config, error) { return LoadWorkspaceConfig(ctx, s.ws) }`. At the start of each tick, `reloadConfig` calls it: an error (including a subscriptions error) keeps the current config and logs once per distinct message; an unchanged config (`reflect.DeepEqual` on Tasks) does nothing; a changed config is stored and logged with the task names added and removed.
- `Run` no longer returns early with no tasks when reload is on; it ticks.
- `StartBackground` calls `WatchConfig` and starts even with zero tasks; `rela scheduler` calls it too.

Alternatives: fsnotify (rejected: misses the sqlite config layer and network
filesystems); rebuilding the scheduler per change (rejected: handlers are
registered once on the queue).

**Files to modify:**
internal/scheduler/{scheduler.go,foreach.go,subscriptions.go,reload.go,reload_test.go},
internal/cli/scheduler.go, internal/appbuild/subscription_e2e_test.go,
docs/feed-subscriptions.md, docs-project GUIDE-scheduled-tasks and
docs/scheduled-tasks.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** operator config files only, through the same
loader and validation as at start. An invalid file is never applied.

**Security-Sensitive Operations:** none new. A task's `run_as` and a
subscription's principal are validated by the same loader.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria.

**Edge Cases:** a file deleted (no tasks; the scheduler idles); the same error
twice (one log); an error then a fix (logged as recovered); a reload during a
run (the handler sees either config, both valid); a race between tick and a job
handler (atomic load).

**Negative Tests:** invalid `schedules.yaml` and invalid `subscriptions.yaml`
both keep the old config.
