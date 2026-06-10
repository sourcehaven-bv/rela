---
id: TKT-9Y4ZWS
type: ticket
title: 'Hot-path benchmarks: dry-run validation, affordances, search, write validation'
kind: test
priority: low
effort: s
status: planning
---

## Problem

The repo has 2 benchmarks total while its own tests document performance
contracts in prose: "dry-run validation must not scan the store — per-keystroke
UI cost" (entitymanager), per-request affordance/`_actions` resolution with
memoization (affordances), the search endpoint, and write-path validation (incl.
fresh-Lua-state-per-rule cost). None of those contracts has a number; "is this
slow?" has no measurable answer.

## Approach (agreed with reviewer in session)

Four benchmark files next to the code they measure, reusing the packages'
existing test fixtures; `just bench` recipe. NO CI regression tracking
(benchstat infra is its own project) — the goal is measurability.

1. `entitymanager`: BenchmarkValidateCreate against a 1000-entity store — if ns/op ever tracks store size, the documented no-scan contract broke.
2. `affordances`: field + relation verdicts through the real Declarative/member-of path (UC10 fixture shape).
3. `search`: bleve text query + hydration over a 1000-entity index.
4. `validation`: Check with when/then rules and with a Lua rule (the per-write fresh-LState cost), 100 entities.

Per session decision: separate PR (3 of 3 split from the original combined
CI-quality task).

## Verification

- Benchmarks run green via `just bench`; lint clean; `just ci` green (benchmarks don't run in CI tests but must compile).
