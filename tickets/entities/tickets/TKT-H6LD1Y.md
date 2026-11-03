---
id: TKT-H6LD1Y
type: ticket
title: Move the live-view hub to internal/livequery
kind: chore
priority: low
effort: m
status: backlog
---

## Description

The live-view hub (subscription registry, worker, rounds, `replayHash`) lives in
`internal/dataentry/live.go`. It depends only on `http.Handler`, `principal` and
a change feed, so it can move to its own package, `internal/livequery`, with
methods `Register`, `Unregister`, `Replace`, `Baseline` and `Run(ctx, feed)`,
the replay handler injected. dataentry keeps the PUT adapter, `parseLivePath`,
the replay chain and the stash middleware. Add an arch-lint rule that
`livequery` does not import dataentry, and let the caller own the worker
goroutine (`Run`) in place of `start`/`stop`.

Do this together with the multi-instance work of FEAT-GWF8YH: forwarding a
subscription replace to the instance holding the stream belongs in that package,
and the per-identity cap then needs shared state (today it counts per instance).

From the architecture review of TKT-HJ5Z9Y (RR findings 4 and 6).
