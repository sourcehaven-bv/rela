---
id: PLAN-QM70WRR
type: planning-checklist
title: 'Planning: Flows M0: extract the session engine into internal/flow'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: a new package `internal/flow` that owns the session engine core, with no
behavior change.

- Instance state: `flow.Instance` (was `channels.Session`),
`flow.ReminderRecord`, `flow.Move`, `flow.Schedule`, `flow.Instance.Enter` (the
move rule of `channels.ApplySession`), `flow.NormTime`.
- State statuses: `flow.Status` (was `realtimecfg.SessionStatus`).
- Periods and timer anchors: `flow.Period`, `flow.ParsePeriod`, `flow.Since`.
- Branch choice: `flow.Decide`, the first-match rule over guarded
branches: poster rule, then guard, then gate. It is source-agnostic: the caller
passes the branches an event selects and a guard callback, so a later event
source (step completion, entity change, job result, manual start) binds its own
guard variables. Guard binding (`GuardHolds`) stays in `realtimecfg`, so `flow`
does not import the predicate engine.
- Timers and reminders: current state (`flow.Read`, stranded handling),
timer due times, reminder slots, the due-time hint, the schedule after a step,
carried restart anchors and reminder records, resume after reopen.
- The due wake and the timer-round schedule (`flow.DueWake`,
`flow.RoundSchedule`).
- The engine reads a `flow.Definition`: a struct of states (name, final,
timers), reminders and the initial state, built once by the `realtimecfg` loader
(`Session.Flow`). `realtimecfg.SessionTimer` and `realtimecfg.SessionReminder`
embed `flow.Timer` and `flow.Reminder` (built with `flow.NewReminder`).
- `flow.Read` returns a `flow.Current`: the stored instance, or the initial
state for an unset one with the stored restart anchors and reminder records
carried over, plus the creation time and the stranded flag. Every engine
function takes `Current`.
- Old names stay as aliases and thin wrappers, so every caller and the
existing test suites compile unchanged.

Out (deferred to later milestones, with reasons):

- The timer loop driver (`realtime.Timers`), outputs runner, trigger
engine, inbox, retention sweep and waiting views stay in `internal/realtime`.
They read the channel store, ACL and message types; moving them needs the run
store of M1 to give them a second caller. Their pure decisions call into
`internal/flow`.
- The store interface (`channels.Store.Append` / `SetSession` with a
`SessionStep`) stays in `internal/channels`: it is the channel store's
transaction contract. The run store of M1 gets its own.
- The `realtime.yaml` loader and load checks stay in `internal/realtimecfg`.

**Acceptance Criteria:**

1. The realtime, realtimecfg and channels test suites pass unchanged
(`go test ./internal/realtime/... ./internal/realtimecfg/...
./internal/channels/...`).
2. `go test -race ./...` passes.
3. `just arch-lint` passes with `internal/flow` below `internal/realtime`,
`internal/realtimecfg` and `internal/channels`.
4. `just seqtrace-compare` shows no change in request flows other than
calls that now pass through `internal/flow`.
5. `internal/flow` has its own unit tests for the moved logic.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: pure refactor; the feature FEAT-J0O10QI records the decision to extract)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: moving existing code, not adding a capability)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal refactor)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `internal/statemachine` is the enum `transitions:` engine; M5 compiles it
onto `internal/flow`, so M0 does not touch it.
- Consumer-side interfaces: `autocascade.Host`, `scheduler.WorkspaceProvider`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Create `internal/flow` (deps: realtimelimits only).
2. Move the types and pure functions listed under Scope; leave aliases
(`type Session = flow.Instance`, ...) and one-line wrappers in the old packages.
3. Rewrite `realtime/timers.go`, `reminders.go`, `session.go` (decider) and
`duewake.go` to call `flow`. The realtime unexported helpers the tests call
(`nextSlot`, `nextTimer`, `stepSchedule`, `resumeSession`, `reminderCounters`,
`sessionDecider`) keep their signatures.
4. Register `flow` in `.go-arch-lint.yml`; `channels`, `realtimecfg` and
`realtime` may depend on it.
5. Document the package in its package doc comment. `docs/` is generated
from `docs-project/`, and there is no user-facing change.

Alternatives rejected:

- Move all of `internal/realtime` session code at once: about 10,000 lines
coupled to the channel store, ACL and message types. No second caller exists
yet, so the boundary would be guessed. Rejected for M0.
- Copy the logic into `flow` and leave the old code: two engines, which the
feature rules out.
- Move `realtimecfg.Session` itself into `flow`: drags outputs, buttons and
ACL grants along. The interface keeps `flow` small.

**Files to modify:**

`internal/flow/*` (new), `internal/channels/session.go`,
`internal/channels/channels.go`,
`internal/realtimecfg/{session,reminders,period,waiting}.go`,
`internal/realtime/{session,timers,reminders,duewake}.go`, `.go-arch-lint.yml`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

No new input. Config validation stays in `realtimecfg`; store-boundary
validation stays in `channels`. Guards keep failing closed.

**Security-Sensitive Operations:**

The poster rule and role checks keep running before the guard, in the same
order, so a trigger still cannot fire a role-limited branch.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. Existing suites unchanged: realtime, realtimecfg, channels, channelstest
backends (file, sqlite, pg when available), dataentry channel handlers.
2. New `internal/flow` unit tests: `Read` (unset with stored records, declared, stranded),
`TimerDue` (since state, since session, restart anchor), `NextSlot` (first,
after last, reload moved first, max reached), `Decide` (first match, poster
refused, guard error stops, gate stops), `Instance.Enter` (re-entry keeps
previous), `DueWake`, `RoundSchedule`.

**Edge Cases:**

- Unset session with stored reminder records: the engine keeps reading the
stored records, not the synthesized initial state.
- Re-entry keeps `Previous`.
- Paused reminders have no slot.

**Negative Tests:**

- A guard that fails to evaluate stops the decision and reports the branch.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A subtle semantic drift in the moved logic. Mitigation: move code
verbatim, keep the existing suites unchanged, add flow unit tests.
- seqtrace shows new edges through `flow`. Mitigation: confirm they are the
only change.

Effort: xl (ticket); this slice: l.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal refactor, no user-facing change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor ticket)

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed (the package doc
comment describes the engine)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-6PD9F3X, RR-9POLWPT, RR-ECU8MZ4, RR-ETZJ9U2,
RR-GXUNEH9, RR-I5C8N00, RR-J2IB8WG, RR-Y3W2UPK (all addressed in the plan)
