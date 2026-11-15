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

Merge-queue runs take 14 to 20 minutes, and the Test job is one of the jobs that
set that length. The race detector makes the slowest packages 6 to 11 times
slower (`internal/dataentry`: 492 s with `-race`, 85 s without).

## Changes

- PR and merge-queue runs execute the Test job's suite without `-race`.
- `scripts/ci-race-gate.sh` runs the concurrency tests under `-race` in the
same job, so they still gate the merge.
- A push to `develop` or `main` runs the whole suite under `-race`. The
`post-merge-alert` job files an issue when that run or Postgres Backend fails,
and closes it when a later push passes.
- Postgres Backend keeps `-race`. Its pgstore step took 366 s without the
detector and 334 s with it, so its time goes to the database.
- Postgres Backend runs as parallel jobs: a build job, and a matrix that splits
the database suites into four parts (pgstore in three by test name, the other
suites in one). A job named "Postgres Backend" stays the required check and
passes only when every part passed.

## Not done

Skipping E2E by changed paths was considered and dropped. `rela-server` is built
from 164 of the 223 Go packages, and none of the last 15 merges would have
skipped it. Frontend tests read `tickets/entities`, which almost every PR
changes, so a Frontend skip would not fire either.
