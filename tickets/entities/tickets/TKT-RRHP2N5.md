---
id: TKT-RRHP2N5
type: ticket
title: 'Flows M0: extract the session engine into internal/flow'
kind: refactor
priority: medium
effort: xl
started: "2026-11-05"
completed: "2026-11-06"
status: done
---

## Description

Extract the channel-session engine into `internal/flow` with no change in
behavior. A channel session becomes a flow whose event source is that channel's
messages.

Scope: the session parts of `internal/realtime` (session, timers, duewake,
outputs, triggers, reminders, inbox waiting), `internal/realtimecfg` (session
config, branches, load checks) and `internal/channels` (`Session`,
`SessionStep`, waiting, the file/pg/sqlite session storage).

Shape the engine for later milestones: event sources are pluggable (message,
step completion, entity change, timer, job result, manual start), and the
session key can later become a run id.

## Acceptance

- The existing realtime and channels test suites pass unchanged.
- `just seqtrace-compare` shows no change in request flows.
- `just arch-lint` passes with `internal/flow` below `internal/realtime`.

Size: `internal/realtime` is about 34,000 lines. Needs `/design-review` before
implementation.
