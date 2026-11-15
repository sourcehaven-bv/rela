---
id: TKT-4RFZMWY
type: ticket
title: Run the full race suites after merge, not in the merge queue
kind: chore
priority: medium
effort: s
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Description

Merge-queue runs take 14 to 20 minutes, and the Test and Postgres Backend jobs
set that length. The race detector makes the slowest packages 6 to 11 times
slower (`internal/dataentry`: 492 s with `-race`, 85 s without).

## Changes

- PR and merge-queue runs execute the Test job's suite and the Postgres
Backend suites without `-race`.
- A step in each of those jobs runs the tests named `Test*Concurren*`,
`Test*Race*` or `Test*Parallel*` under `-race`, so concurrency tests still gate
the merge.
- A push to `develop` or `main` runs every suite under `-race`. The
`post-merge-alert` job files an issue when that run fails.

## Not done

Skipping E2E by changed paths was considered and dropped. `rela-server` is built
from 164 of the 223 Go packages, and none of the last 15 merges would have
skipped it. Frontend tests read `tickets/entities`, which almost every PR
changes, so a Frontend skip would not fire either.
