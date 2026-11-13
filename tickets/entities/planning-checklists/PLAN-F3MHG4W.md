---
id: PLAN-F3MHG4W
type: planning-checklist
title: 'Planning: Weekly postgres scale run'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a weekly and manually triggered GitHub workflow on postgres;
`tools/scalerun` (Go driver, checker and markdown report); `just scale-run`;
size-parameterised benchmarks for pgstore GraphQuery, pgstore search, bleve
search and validation; a guide page stating the supported postgres scale. Out:
fs and sqlite backends (single-process, small scale); blocking PRs; fixing the
slow endpoints the run finds (follow-up tickets).

**Acceptance Criteria:**
1. `scalerun run` seeds the perf project at scale 1 and 5 into its own schema, starts rela-server with `-verbose`, and records p50/p95, SQL statement count and RSS per endpoint and principal. Test: local run against Postgres.app.
2. `scalerun check` fails on a p95 over its limit, a scale ratio over 2x, a benchmark regression over 20%, or a recorded error. Test: `TestCheck`.
3. A failing scheduled run opens or updates one GitHub issue; a passing run closes it. Test: actionlint; the script follows archplan-weekly.yml.
4. The report goes to the job summary and an artifact. Test: `TestWriteReport` and a local `check`.
5. A guide states the supported scale. Test: `just docs` output.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the ticket fixes the approach; no open design choice)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo precedents cover it)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- benchstat (golang.org/x/perf) for benchmark comparison; chosen over a hand-rolled comparison because it gives significance.
- `rela dev seed` and `prototypes/perf/project` provide the data and principals.
- `tools/seqtrace/demo/run.sh` starts rela-server on postgres with a principal header; the same approach is used here.
- The `Server-Timing` header under `-verbose` already reports per-request query counts.
- `security.yml` and `archplan-weekly.yml` give the issue open/update/close pattern.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** A Go tool in `tools/scalerun` with `run` and `check`
subcommands, configured by `tools/scalerun/scalerun.yaml` (endpoints,
principals, limits). `run` uses one schema per scale, seeds a copy of the perf
project, starts the headless postgres server, warms up, then measures each
endpoint per principal over HTTP. `check` compares with the limits and with the
benchstat CSV, and writes `report.md`. `just scale-run` builds, runs, benchmarks
and checks; the workflow runs that recipe and handles cache, summary, artifact
and issue.

Alternatives: a shell script with curl (rejected: hard to test, no percentile
logic); k6 (rejected: extra dependency, no access to Server-Timing parsing in
the same report); pg_stat_statements for query counts (rejected: Server-Timing
already gives per-request counts).

**Files to modify:** `tools/scalerun/**`, `.github/workflows/scale-run.yml`,
`justfile`, `.testcoverage.yml`, the three bench files, new
`internal/store/pgstore/search_bench_test.go`, a new guide in `docs-project` and
generated `docs/`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The config file (validated on load: method
allowlist, path shape, known principals, numeric limits); the `scales` workflow
input (regex `^[0-9.,]+$` in the workflow, range check in `parseScales`, passed
via env, never spliced into a script); `RELA_DATABASE_URL` from the environment
only.

**Security-Sensitive Operations:** The tool drops and recreates schemas named
`scalerun_s<scale>`, so it must point at a scratch database; the docs say so.
The issue job is the only one with `issues: write`. The DSN never appears as a
flag.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Unit tests for config parsing, percentiles, Server-Timing
parsing, the driver against a fake server, benchstat CSV parsing, `Check` and
the report. Integration: a local run at scale 1 and 5 against Postgres.app.

**Edge Cases:** No baseline on the first run; benchstat `~` (not significant); a
missing Server-Timing header (server without `-verbose`); `{created}` without an
earlier `{new}`; a p95 below the ratio floor.

**Negative Tests:** Invalid config values each fail with a named error; a 403 is
recorded as an error, not a sample; an unreachable server aborts the run.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Runner noise causes false failures: limits have about 3x headroom and
the ratio check ignores endpoints under a floor. The workflow cannot be run
before merge: it is validated with actionlint and the recipe it calls was run
locally. Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] New guide `docs/postgres-scale-run.md` (from docs-project GUIDE-postgres-scale-run)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the requester fixed the design in the ticket; the code review covers the result)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review was run)

**Design Review Findings:** N/A
