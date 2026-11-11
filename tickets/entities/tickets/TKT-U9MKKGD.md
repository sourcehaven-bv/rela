---
id: TKT-U9MKKGD
type: ticket
title: Flows docs and CLI promise retries that do not exist (#2170)
kind: docs
priority: medium
effort: s
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

## Description

The flows docs and CLI promise automatic retries that this release does not make
(sourcehaven-bv/rela#2170). Retries and timers run through
`flowengine.Engine.Due`, and outside tests nothing calls it; the worker comes
with TKT-FWQZGAM. A step that fails during a start therefore stays pending until
someone retries it in the web app or cancels the run.

- GUIDE-flows: say in the status banner and in "Running flows" that failed
steps and timers wait for the worker, and that a pending run is retried by hand
(the Retry button or `POST .../_retry`) or ended with `rela flow cancel`.
- `rela flow start`: replace "will be retried" with how to retry or cancel.
- The web app, the MCP tool description, GUIDE-mcp-server and
`flowview.Completed.Problem` stop promising an automatic retry or a next
attempt.

## Acceptance

- No doc, CLI message, MCP description or web app text says a failed flow
step is retried automatically.
