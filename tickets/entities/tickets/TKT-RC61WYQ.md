---
id: TKT-RC61WYQ
type: ticket
title: Bound golangci-lint memory and serialise runs
kind: chore
priority: medium
effort: xs
started: "2026-11-02"
completed: "2026-11-02"
status: done
---

## Problem

Each golangci-lint run type-checks the whole module and can use several GB of
RAM. Agents often started lint runs in parallel shells, which exhausted memory
on developer machines.

## Change

- Set `run.concurrency: 4` in `.golangci.yml` to cap parallel package analysis.
- Run golangci-lint in the `justfile` with `GOMEMLIMIT` (default `1500MiB`) and
`--allow-serial-runners`, so a second run waits for the first.
- Tell agents in `CLAUDE.md` to run lint one at a time.
