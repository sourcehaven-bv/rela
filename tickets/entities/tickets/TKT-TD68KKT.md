---
id: TKT-TD68KKT
type: ticket
title: 'neoq fork: clean up redis start failures and memory shutdown order'
kind: chore
priority: low
effort: xs
status: backlog
---

## Description

Follow-up from the code review of TKT-736J0CZ (sourcehaven-bv/neoq#3). Three
gaps in the fork remain:

1. Redis `Backend` now returns an error when `mgr.Start` or `server.Start` fails, but leaves the client, the inspector and the server connection open. A caller that retries against an unavailable redis gains connections. The remaining `log.Fatal` after `NewPeriodicTaskManager` should also become a returned error.
2. The memory backend cancels its consumers before it stops cron, so a cron tick in that window can block on a full queue channel. Stop cron first. Postgres has the same order.
3. The memory backend comment says cron v1 `Stop` waits for the goroutine to exit. It only waits until the goroutine receives the signal, and two concurrent `Shutdown` calls deadlock on the unsynchronized `running` flag. Guard `Shutdown` with `sync.Once` and fix the comment.

rela uses neither the redis backend nor neoq cron, so none of these affects rela
today.

## Acceptance criteria

1. Each gap has a fork test, and the fork PR is reviewed before it merges.
2. The go.mod replace is bumped and the patch list updated.
