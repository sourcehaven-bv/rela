---
id: TKT-UHKZKOB
type: ticket
title: Weekly postgres scale run
kind: enhancement
priority: medium
effort: m
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

## Description

Scheduled weekly workflow on postgres: `rela dev seed` at scale 1 (~20k
entities) and scale 5 (~100k), call about 15 key endpoints (list, kanban, gantt,
search, dashboard, entity detail, a few writes) as several principals, record
p95 latency, SQL query count and memory, and run size-parameterised benchmarks
compared with the previous run via benchstat.

Fails when p95 exceeds a set limit, scale-5 time is more than ~2x scale-1 time,
or a benchmark regresses more than 20%. Failures open a GitHub issue; PRs are
never blocked. fs and sqlite are out of scope (single-process, small scale).

## Done when

- The workflow runs green.
- A guide page states the supported postgres scale that the job defends.
