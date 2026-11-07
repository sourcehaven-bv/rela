---
id: TKT-L98KKVX
type: ticket
title: Stop the neoq memory backend's cron goroutine on Shutdown
kind: chore
priority: low
effort: s
status: backlog
---

## Description

The neoq memory backend starts a robfig/cron runner per backend and its Shutdown
never calls cron.Stop (the KNOWN LEAK on jobs.neoqQueue.Close). Every appbuild
Services on a non-postgres build therefore leaves one idle goroutine behind
after Close.

`rela test` (FEAT-IODR9ST) builds one world per case with appbuild.NewInMemory,
so a run leaks one goroutine per case. TestNewInMemory_CloseStopsEveryGoroutine
exempts that goroutine until this is fixed.

## Acceptance criteria

- The sourcehaven-bv/neoq fork stops the cron runner in the memory backend Shutdown, and go.mod points at it.
- The goleak exemption in internal/appbuild/memrecipe_test.go and the KNOWN LEAK note in internal/jobs/neoqqueue.go are removed.
