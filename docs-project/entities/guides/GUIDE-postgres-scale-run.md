---
id: GUIDE-postgres-scale-run
type: guide
title: PostgreSQL Scale Check
status: published
summary: The PostgreSQL scale that rela checks every week, and how to run the check yourself
order: "32"
audience: advanced
---

rela checks every week how the PostgreSQL backend behaves with a large dataset.
This page states the scale that check defends, what it measures, and how to run
it yourself.

## Supported scale

The PostgreSQL backend is checked at up to **about 100,000 entities and 240,000
relations** (seed scale 5 of the perf project). At that size every measured
endpoint stays within its limit in `tools/scalerun/scalerun.yaml`.

Measured on 2026-11-13 on a laptop (p50, scale 1 and scale 5):

| Endpoint | 19k entities | 96k entities |
| --- | ---: | ---: |
| List page with relation columns | 9 ms | 9-17 ms |
| Kanban column (100 cards) | 13 ms | 42 ms |
| Entity detail | 3-5 ms | 2 ms |
| View with table sections | 7-30 ms | 3-22 ms |
| Create, update, delete | 14-26 ms | 1-4 ms |
| Dashboard query card | 53-140 ms | 270-500 ms |
| Next action | 33 ms | 210 ms |
| Full-text search | 250-520 ms | 1.4-2.9 s |
| Gantt chart | 165 ms | 880 ms |

The gantt row was measured on 2026-11-14.

Paged reads and writes do not get slower as the data grows. Search, dashboard
query cards, the next-action suggestion and the gantt chart still read data in
proportion to its size, so they grow with it. Follow-up tickets track each of
them; until they land, the check holds them to their current growth so they
cannot get worse unnoticed.

The filesystem and SQLite backends are not part of this check. They are
single-process backends meant for smaller datasets.

## What the check measures

The **Scale run** workflow (`.github/workflows/scale-run.yml`) runs every
Saturday and can be started by hand. It:

1. seeds `prototypes/perf/project` into PostgreSQL with `rela dev seed` at
scale 1 and scale 5, each in its own schema;
2. starts `rela-server` with `-verbose` and calls about fifteen endpoints:
lists, a kanban column, the gantt chart, search, dashboard cards, the
next-action suggestion, entity detail, views, and create, update and delete. It
calls them as a manager, an editor and a reader from the project's `acl.yaml`;
3. records p50 and p95 latency, the number of SQL statements per request and
the server's memory (RSS);
4. runs benchmarks at two data sizes for the pgstore graph query, pgstore
search, in-memory search and validation, and compares them with the last passing
run using `benchstat`.

The run fails when:

- an endpoint's p95 is over its limit for that scale;
- an endpoint's p50 at scale 5 is more than twice its p50 at scale 1, unless
the endpoint has its own `ratio_max` or is under 50 ms;
- a benchmark is more than 20% slower than in the last passing run;
- a request returns an unexpected status.

A failing run opens a GitHub issue titled "Scale run: postgres limits exceeded",
or updates it if it is already open. The first passing run closes it. The run
never blocks a pull request. Each run writes its report to the job summary and
uploads it, with the raw results and logs, as the `scale-run` artifact.

## Run it locally

You need a PostgreSQL server you can create schemas in, and `benchstat`:

```bash
go install golang.org/x/perf/cmd/benchstat@latest
createdb rela_scale
export RELA_DATABASE_URL='postgres://localhost:5432/rela_scale?sslmode=disable'
just scale-run     # scale 1 and 5; about 30 minutes
just scale-run 1   # scale 1 only, no ratio check; about 10 minutes
```

Use a scratch database. The run creates and drops the schemas `scalerun_s1` and
`scalerun_s5` in it. The database must allow `CREATE EXTENSION pg_trgm`, or
already have it.

The report is in `.ignored/scalerun/out/report.md`. A passing run saves its
benchmark output as the baseline for the next local run, in
`.ignored/scalerun/bench-baseline.txt`.

## Change a limit

Limits are in `tools/scalerun/scalerun.yaml`. Each p95 limit is about three
times the p95 measured on a laptop, because a shared CI runner is slower and
noisier. Change a limit when the runner or the seed data changes, or when a
change makes an endpoint faster and you want the check to hold the gain. Do not
raise a limit only to make a failing run pass: find out why the endpoint got
slower first.

A benchmark has no limit to change: it is compared with the last passing run on
the default branch. When a change makes a benchmark slower on purpose, start the
workflow by hand with `reset_baseline`. That run skips the comparison and, if
the endpoints pass, becomes the new baseline.
