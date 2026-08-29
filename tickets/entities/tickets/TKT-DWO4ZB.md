---
id: TKT-DWO4ZB
type: ticket
title: 'Reachability floor: merged-coverage pipeline + scupper (report-only baseline)'
kind: enhancement
priority: medium
effort: m
status: done
---

## Scope

Establish a **reachability floor**: every line of Go runs at least once under
*some* test — unit, cross-package (`-coverpkg`), postgres-tagged, or e2e — or is
explicitly dismissed with a reasoned directive in the source.

Reachability is a **floor, not a quality gate**. "Executed at least once" is
strictly weaker than "tested": a line an e2e flow merely walks past counts as
reached. The value is catching code that has *never run under any test* — dead
branches, unreachable-in-practice paths — which is unobserved until production.
Test quality is a separate axis built on top of this floor.

This ticket covers the **report-only first pass**. No threshold is enforced; it
establishes an honest baseline. A rising per-package floor is deliberately left
to later work.

## What lands

1. `scripts/reachability.sh` — collects coverage from every source as Go binary
   coverage (`GOCOVERDIR` / `-cover`), merges with `go tool covdata` (which
   unions counters, so "reached by ANY source" is measured), and reports via
   `scupper`.
2. A `just reachability` recipe and a **non-blocking** CI job.
3. `scupper` pinned at `v0.2.0`, invoked with `-d coverage-ignore
   --require-reason`.
4. Supporting changes required to *collect* the coverage at all:
   - **rela-server graceful shutdown** — a `-cover` binary only writes its
     counters on a clean exit, so a server that is killed loses all e2e
     coverage. This is a prerequisite, not an incidental refactor.
   - e2e `-cover` wiring (`RELA_E2E_COVERDIR`) in the Playwright fixtures.

## Directive syntax — why `coverage-ignore`

`scupper`'s directive keyword is configurable via `-d`. Two spellings are
therefore both functional: its own default `//scupper:ignore` and the
go-test-coverage style `// coverage-ignore:`.

This work standardises on **`coverage-ignore`** because the repo already runs
`vladopajic/go-test-coverage` against `.testcoverage.yml`, which has
`force-annotation-comment: true` and understands exactly that spelling. One
annotation vocabulary serves both tools; `//scupper:ignore` would be a second,
tool-specific dialect meaning the same thing.

Directives in this branch: 16 `// coverage-ignore:` (single line) and 25
`// coverage-ignore-func:` (whole function). `--require-reason` makes an
unexplained dismissal a hard error, so every exclusion stays reviewable in the
diff.

## Measured baseline

Against `develop` (unit + cross-package legs; e2e and postgres legs opt-in and
omitted from CI to keep the job fast):

```
reachability: 77.1% of statements executed (36805/47755)
dismissed: 333 statement(s) excluded via coverage-ignore directives
8411 line-block(s) never executed by any test
```

Report-only: the script exits 0 without `--threshold`.

## Relationship to `.testcoverage.yml`

This does **not** replace the existing per-package coverage thresholds. They
answer different questions:

- `.testcoverage.yml` (go-test-coverage) — *how well* is a package tested,
  per-package percentage floors, from the standard unit-test profile.
- reachability (scupper) — has each line run **at all**, under *any* test
  source, from a merged/generous profile.

A package can sit comfortably above its coverage threshold while still
containing lines no test has ever executed.

## Acceptance criteria

- AC1 `scripts/reachability.sh` merges unit + cross-package coverage and reports
  a reachability number; postgres and e2e legs are included when
  `RELA_TEST_DATABASE_URL` / `RUN_E2E=1` are set.
- AC2 The CI job is report-only: it does not fail the build and is not a
  required check.
- AC3 Every dismissal carries a reason (`--require-reason` passes).
- AC4 rela-server shuts down gracefully on SIGTERM/SIGINT so its coverage is
  flushed.
- AC5 `go build ./...` and `go test ./...` pass.

## Follow-up (explicitly NOT in this ticket)

- Turning on a threshold / rising per-package floor.
- Annotating the remaining never-executed blocks. A large body of candidate
  `defensive:` / `os-fs-event:` / `main-or-wiring:` dismissals exists and can be
  landed incrementally once this baseline is in place.
