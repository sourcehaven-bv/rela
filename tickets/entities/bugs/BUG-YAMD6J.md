---
id: BUG-YAMD6J
type: bug
title: A job queue closing in another process stops rela-server's job processing
description: neoq's Shutdown pg_notify'd a sentinel job ID that every listener on the database received; any rela-postgres CLI command silently stopped rela-server's listener and no job ran again until a restart.
priority: critical
effort: s
why1: 'No scheduled task ran on Atlas after a rela-postgres CLI command: every run expired with ''lease expired while queued''.'
why2: rela-server's neoq listener goroutine had returned; notifications for new jobs were never read and the ten workers waited forever.
why3: It returned on payload '-1' which neoq's Shutdown pg_notify'd to stop its own listener. NOTIFY reaches every session listening on the channel including other processes.
why4: Every rela-postgres CLI command builds and starts and closes a queue (buildJobQueue) so routine CLI use and deploys sent the sentinel to the running server.
why5: 'The queue''s conformance suite exercises one process per database. No test ran a second process against the same database: the normal production shape.'
prevention: TestPostgresQueue_SurvivesAnotherProcessClosing runs a second queue on the same database and asserts the first keeps processing. The fork carries TestShutdownDoesNotStopOtherBackends.
started: "2026-10-03"
completed: "2026-10-03"
status: done
---

## Summary

neoq's postgres `Shutdown` stopped its listener by running `pg_notify(queue,
'-1')`. NOTIFY reaches every session listening on that channel, so the listener
of every OTHER process on the database also received `-1`, took it as its own
shutdown, and returned. Its workers stayed idle forever: jobs were still
inserted and announced (the pending-job poll re-announces them every minute),
but nothing read the notifications. Nothing was logged above debug level.

Every `rela-postgres` CLI command builds, starts and closes a job queue (see
`buildJobQueue`), so any CLI command against a server's database stops that
server's job processing until the next restart.

## Impact (Atlas, rela-server-postgres)

- 2026-10-01 13:35:18: a manual `rela-postgres analyze states`. Last job
processed 13:35:18.136.
- 2026-10-02 08:46:16: the deploy's `rela-postgres init` and `migrate`, six
seconds after rela-server restarted on v26.10.0.
- No scheduled task ran from 2026-10-01 until a restart on 2026-10-03: the
weekly validation review, recurrence, MT agenda and the daily deadline mail. The
scheduler logged `lease expired while queued` on every retry.

Evidence: a goroutine dump (SIGQUIT) showed ten idle neoq workers and no
`listen` goroutine; `ss` showed 145 KB unread on the `LISTEN "rela"` socket.

## Reproduction

`TestPostgresQueue_SurvivesAnotherProcessClosing` (internal/jobs): a queue
processes a job, a second queue on the same database starts and closes, and the
first never processes another job. Fails on the previous neoq pin.

## Fix

The `sourcehaven` branch of sourcehaven-bv/neoq:

- `Shutdown` no longer broadcasts; cancelling its own contexts already
interrupts the local `WaitForNotification`. The listener ignores `-1` from older
peers instead of returning.
- `acquire` no longer leaks a goroutine after every error (it sent on two
unbuffered channels with no return between them), nor a connection acquired as
the deadline passed.
- Startup goroutines exit quietly when shut down mid-connect.
- Also carries the JobTimeout fix (code.adriano.fyi/me/neoq/pulls/10) and the
schema-qualified sequence fix (BUG-YJEIFH).

## Not fixed here

On restart, the queue starts before the scheduler registers its handlers, so the
pending backlog is consumed as `no handler registered for kind, dropping`. The
scheduler recovers through its lease and retry ladder. Not filed yet.
