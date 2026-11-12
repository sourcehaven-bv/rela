---
id: TKT-PY2G11W
type: ticket
title: 'Flows M4b: job steps'
kind: enhancement
priority: medium
effort: l
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

## Description

Job steps for flows (part C of the M4 split). Depends on parallel branches
(TKT-ZO9IB6V) and on the flow worker that calls `Engine.Due` (TKT-FWQZGAM),
which drives backoff and per-attempt timeouts.

`job` steps run through `jobs.Queue`. Exhausted retries move the run to
`failed`; an operator can retry it.

## Design (approved)

### Targets

A `job` step names exactly one of:

- `script:`, a Lua script under `scripts/`, run through
`script.NewWriterRuntime`;
- `action:`, a declared `_actions` action, with that action's permission
check;
- `kind:`, a built-in job such as mail or webhook.

### Failure handling

- Never started: always retried.
- An error the job returns is retryable or permanent:
  - A built-in kind classifies its own errors. For HTTP, 5xx, 429 and
network errors are retryable; other 4xx are permanent.
  - Lua can raise a permanent error (for example `retry = false`).
  - Default: 3 attempts, backoff from 10s, factor 2, max 5m.
- Unknown outcome (the attempt timed out, or the worker died): no automatic
retry by default. The run moves to `failed` with that failure kind.

### Escape hatches

- `idempotent: true` also retries timeouts and lost workers.
- Each attempt gets an idempotency key: run id + step + attempt.
- `retry: {attempts, backoff, timeout}` tunes the defaults. `attempts`
counts every execution, the first included. `timeout` is per attempt and counts
from enqueue, so queue time counts.
- `retry: false` fails on the first error.
- Operator retry from `failed` starts a fresh attempt count.

```yaml
job:
  script: notify-hr.lua
  idempotent: false
  retry:
    attempts: 3
    backoff: { initial: 10s, factor: 2, max: 5m }
    timeout: 2m
```

The earlier `on:` list is dropped. Whether an attempt retries follows from the
job's error classification and `idempotent`.

## Acceptance

- Tests cover a failing job that exhausts its retries, a permanent error, a
timed-out attempt, a lost worker, `idempotent: true`, operator retry and the
idempotency key.
