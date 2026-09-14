# Coverage honesty report — rela

Regenerated on 2026-09-12 against `origin/develop` (b34dbae8) using the landed
`scripts/reachability.sh` pipeline from TKT-DWO4ZB. It supersedes the
2026-08-28 snapshot, whose numbers were measured on a 7-week-old tree with a
`scupper:ignore` directive dialect the landed pipeline does not read.

Reachability = executed at least once by ANY test (unit + cross-package +
build-tagged + e2e), NOT tested. It is a floor; test quality is a separate axis.

## How these numbers were produced

```bash
./scripts/reachability.sh          # no --threshold: report-only
```

Legs included in this run: unit + cross-package (`-coverpkg=./...`).
Legs NOT included: **e2e** (needs `RUN_E2E=1` + Playwright browsers) and
**postgres** (needs `RELA_TEST_DATABASE_URL`). Both materially change the
picture — see the caveats. Every figure below is from the merged profile of the
legs that did run, so it is a LOWER bound on reachability.

## Headline

| Metric | Value |
| --- | --- |
| Statements measured | **58,326** |
| Reached by some test | **46,157 (79.1%)** |
| Not reached by any test | 12,169 |
| Statements dismissed by `coverage-ignore` directives | **1,187** |
| Directives in tree (excluding `-end` closers) | 312 |
| Of those, added by this branch | 260 |
| Baseline on `develop` (same profile) | 78.6%, 395 statements dismissed |

The branch's contribution is the classification layer: it raises dismissed
statements from 395 to 1,187 (+792) and reachability from 78.6% to 79.1%. The
reachability delta is small by design — a dismissal removes a statement from
the denominator, it does not make code run.

## The dialect correction

The preserved branch wrote 282 `//scupper:ignore <category>: <reason>`
comments. The pipeline that actually landed invokes scupper with
`-d coverage-ignore`, which reads ONLY `// coverage-ignore*:` comments. Verified
empirically: under `-d coverage-ignore` a `scupper:ignore` comment is not
honoured and its block is reported unreached. All surviving annotations were
therefore converted to the `coverage-ignore` dialect, preserving each reason
verbatim (the category is kept as the first word of the reason).

## Categories of the dismissals added here

| Category | Count | What it is |
| --- | ---: | --- |
| `defensive` | 180 | `if err != nil` on a call that cannot fail with the test backend |
| `os-fs-event` / `os-fs` | 25 | fsnotify, signals, real filesystem/network, Wails runtime |
| `main-or-wiring` | 17 | process entry points, flag/DI startup wiring |
| `unreachable` / `unreachable-default` | 15 | switch defaults and exhaustiveness guards no valid input reaches |
| `panic-invariant` / `invariant` | 8 | panics and returns guarding "cannot happen" invariants |
| `external-tool` | 4 | shells out to `dot`, etc. |
| `postgres-only` | 4 | only reachable on the postgres backend |
| `sealing` | 2 | unexported-marker methods that seal an interface |

Counted by diff line, this is 255 added `coverage-ignore*:` lines; counted as
directives it is 260, because a wrapped reason puts the directive and part of
its text on one line.

## Validation performed

Each dismissal was checked against the merged profile:

- **106 single-line dismissals**: 0 sit on a statement the profile shows as
  reached (column-aware check — a dismissal on an `if` line correctly dismisses
  the body, not the condition).
- **161 `-start`/`-end` blocks**: 112 fully unreached, 27 partially reached
  (the block spans a reached guard plus an unreached body), 19 contain no
  statements. **3 were fully reached and were DROPPED as stale** — the code they
  called unreachable now runs under test:
  `internal/config/config.go` (watcher OnChange closure),
  `internal/predicate/eval.go` (evalOrdered fallback),
  `internal/store/fsstore/watcher.go` (StopWatching nil guard).

## ⚠ Caveats — numbers that need a second look

1. **`internal/store/pgstore` reads 6.8%** and that is a collection artifact,
   not reality. Its suite is gated behind the `postgres` build tag and a live
   database. Re-run with `RELA_TEST_DATABASE_URL` set before drawing any
   conclusion about this package.
2. **`internal/dataentry` (82.9%) and `cmd/rela-server` (10.0%)** are understated
   for the same reason: their HTTP surface is driven by the e2e leg, which was
   not run here. Set `RUN_E2E=1`.
3. **`cmd/rela-desktop` (34.9%)** needs the live Wails runtime; much of the
   remainder is not reachable from `go test` at all.
4. The previous version of this report cited "gap-groups" from a 63-agent
   classifier sweep. Those figures are not reproducible from the pipeline and
   have been removed rather than carried forward unverified.

## Per-package reachability

Packages with unreached statements or directives, worst absolute gap first.
`Dismissed` counts `coverage-ignore*` directives in the package.

| Package | Stmts | Reached | Pct | Unreached | Dismissed |
| --- | ---: | ---: | ---: | ---: | ---: |
| `internal/store/pgstore` | 2715 | 185 | 6.8% | 2530 | 3 |
| `internal/cli` | 3577 | 1382 | 38.6% | 2195 | 36 |
| `internal/dataentry` | 10084 | 8357 | 82.9% | 1727 | 21 |
| `internal/lua` | 3913 | 3408 | 87.1% | 505 | 4 |
| `cmd/rela-desktop` | 691 | 241 | 34.9% | 450 | 20 |
| `internal/datamigration` | 1444 | 1012 | 70.1% | 432 | 0 |
| `internal/mcp` | 1153 | 770 | 66.8% | 383 | 29 |
| `internal/docs` | 1592 | 1230 | 77.3% | 362 | 0 |
| `internal/docscapture` | 497 | 213 | 42.9% | 284 | 1 |
| `internal/git` | 452 | 172 | 38.1% | 280 | 2 |
| `internal/appbuild` | 822 | 595 | 72.4% | 227 | 17 |
| `cmd/rela-server` | 231 | 23 | 10.0% | 208 | 15 |
| `internal/store/fsstore` | 1383 | 1191 | 86.1% | 192 | 6 |
| `internal/cli/sync` | 681 | 496 | 72.8% | 185 | 4 |
| `internal/metamodel` | 2044 | 1866 | 91.3% | 178 | 10 |
| `internal/predicate` | 926 | 750 | 81.0% | 176 | 12 |
| `internal/entitymanager` | 1320 | 1150 | 87.1% | 170 | 12 |
| `internal/migration` | 1154 | 996 | 86.3% | 158 | 2 |
| `internal/dataentryconfig` | 2101 | 1967 | 93.6% | 134 | 5 |
| `internal/store/storetest` | 3403 | 3282 | 96.4% | 121 | 0 |
| `internal/store/sqlitestore` | 798 | 686 | 86.0% | 112 | 0 |
| `internal/testutil` | 255 | 163 | 63.9% | 92 | 0 |
| `internal/visibility/visibilitytest` | 378 | 290 | 76.7% | 88 | 0 |
| `internal/affordances` | 625 | 545 | 87.2% | 80 | 12 |
| `internal/attachment` | 317 | 240 | 75.7% | 77 | 0 |
| `internal/acl` | 1182 | 1109 | 93.8% | 73 | 8 |
| `internal/predicatefns` | 414 | 351 | 84.8% | 63 | 6 |
| `internal/aclmap` | 339 | 278 | 82.0% | 61 | 0 |
| `internal/analysis` | 382 | 321 | 84.0% | 61 | 0 |
| `internal/mail` | 420 | 364 | 86.7% | 56 | 0 |
| `internal/importer` | 262 | 207 | 79.0% | 55 | 0 |
| `internal/scheduler` | 401 | 346 | 86.3% | 55 | 4 |
| `internal/projectsetup` | 203 | 153 | 75.4% | 50 | 5 |
| `internal/search/bleveindex` | 280 | 234 | 83.6% | 46 | 12 |
| `internal/storage` | 490 | 445 | 90.8% | 45 | 11 |
| `internal/schema` | 241 | 198 | 82.2% | 43 | 4 |
| `internal/sqlitedb` | 166 | 123 | 74.1% | 43 | 0 |
| `internal/visibility` | 403 | 362 | 89.8% | 41 | 0 |
| `internal/filter` | 477 | 437 | 91.6% | 40 | 0 |
| `internal/search` | 273 | 234 | 85.7% | 39 | 0 |
| `internal/conflict` | 286 | 248 | 86.7% | 38 | 0 |
| `internal/state/statetest` | 127 | 92 | 72.4% | 35 | 0 |
| `internal/templating` | 212 | 178 | 84.0% | 34 | 2 |
| `internal/aclaudit` | 485 | 452 | 93.2% | 33 | 0 |
| `internal/automation` | 277 | 244 | 88.1% | 33 | 0 |
| `internal/comments/filecomments` | 138 | 111 | 80.4% | 27 | 0 |
| `internal/jobs` | 208 | 182 | 87.5% | 26 | 0 |
| `internal/validation` | 311 | 286 | 92.0% | 25 | 1 |
| `cmd/gen-icons` | 124 | 100 | 80.6% | 24 | 2 |
| `internal/imgproc` | 202 | 178 | 88.1% | 24 | 0 |
| `internal/mailrender` | 263 | 239 | 90.9% | 24 | 0 |
| `internal/appbuild/appbuildtest` | 103 | 80 | 77.7% | 23 | 0 |
| `internal/cmdexec` | 186 | 163 | 87.6% | 23 | 0 |
| `internal/script` | 219 | 196 | 89.5% | 23 | 0 |
| `internal/store/graphquerynaive` | 224 | 201 | 89.7% | 23 | 0 |
| `internal/computed` | 121 | 99 | 81.8% | 22 | 0 |
| `internal/jwtauth` | 97 | 75 | 77.3% | 22 | 1 |
| `internal/markdown` | 430 | 409 | 95.1% | 21 | 4 |
| `internal/userstate/kvuserstate` | 157 | 137 | 87.3% | 20 | 0 |
| `internal/calfeed` | 215 | 197 | 91.6% | 18 | 0 |
| `internal/perfseed` | 329 | 311 | 94.5% | 18 | 0 |
| `internal/autocascade` | 116 | 99 | 85.3% | 17 | 2 |
| `internal/ai` | 314 | 298 | 94.9% | 16 | 4 |
| `internal/apiwire/v1` | 91 | 75 | 82.4% | 16 | 8 |
| `internal/entity` | 217 | 201 | 92.6% | 16 | 2 |
| `internal/jobs/jobstest` | 303 | 287 | 94.7% | 16 | 0 |
| `internal/statemachine` | 200 | 184 | 92.0% | 16 | 9 |
| `internal/transform` | 113 | 97 | 85.8% | 16 | 0 |
| `internal/store/memstore` | 548 | 533 | 97.3% | 15 | 1 |
| `internal/tenant` | 141 | 126 | 89.4% | 15 | 0 |
| `internal/tracer` | 116 | 101 | 87.1% | 15 | 0 |
| `internal/store` | 160 | 148 | 92.5% | 12 | 0 |
| `internal/renametype` | 69 | 58 | 84.1% | 11 | 0 |
| `internal/schedulerstate/kvstate` | 94 | 83 | 88.3% | 11 | 0 |
| `internal/comments` | 121 | 111 | 91.7% | 10 | 0 |
| `internal/config` | 134 | 124 | 92.5% | 10 | 1 |
| `internal/openapi` | 228 | 218 | 95.6% | 10 | 1 |
| `internal/queryplan` | 140 | 130 | 92.9% | 10 | 0 |
| `internal/config/configsql` | 69 | 60 | 87.0% | 9 | 0 |
| `internal/desktop` | 45 | 36 | 80.0% | 9 | 1 |
| `internal/entitymanager/entitymanagertest` | 9 | 0 | 0.0% | 9 | 0 |
| `internal/mailtemplate` | 76 | 67 | 88.2% | 9 | 0 |
| `internal/search/searchparser` | 125 | 116 | 92.8% | 9 | 0 |
| `internal/nextaction` | 182 | 174 | 95.6% | 8 | 0 |
| `internal/output` | 187 | 179 | 95.7% | 8 | 3 |
| `internal/audit` | 109 | 103 | 94.5% | 6 | 2 |
| `internal/canonical` | 120 | 114 | 95.0% | 6 | 1 |
| `internal/docscli` | 43 | 37 | 86.0% | 6 | 0 |
| `internal/secrets` | 65 | 59 | 90.8% | 6 | 0 |
| `internal/userstate/memuserstate` | 90 | 84 | 93.3% | 6 | 0 |
| `internal/conditionlint` | 102 | 97 | 95.1% | 5 | 0 |
| `internal/project` | 102 | 97 | 95.1% | 5 | 1 |
| `internal/validator` | 37 | 32 | 86.5% | 5 | 0 |
| `internal/worldreader` | 74 | 69 | 93.2% | 5 | 0 |
| `internal/app` | 15 | 11 | 73.3% | 4 | 0 |
| `internal/mermaid` | 93 | 89 | 95.7% | 4 | 0 |
| `internal/principal` | 58 | 54 | 93.1% | 4 | 0 |
| `internal/caldavalias` | 80 | 77 | 96.2% | 3 | 0 |
| `internal/comments/memcomments` | 62 | 59 | 95.2% | 3 | 0 |
| `internal/pattern` | 83 | 80 | 96.4% | 3 | 2 |
| `internal/state/statesql` | 19 | 16 | 84.2% | 3 | 0 |
| `internal/store/storeutil` | 226 | 224 | 99.1% | 2 | 0 |
| `internal/worlds` | 84 | 82 | 97.6% | 2 | 0 |
| `internal/mail/mailtest` | 105 | 104 | 99.0% | 1 | 0 |

## Path to a real floor

1. **Measure all legs.** Run `just reachability` with `RUN_E2E=1` and
   `RELA_TEST_DATABASE_URL` set. pgstore, dataentry and rela-server are all
   understated until then; a threshold set against today's partial number would
   encode the measurement gap as if it were a code property.
2. **Then re-classify.** What remains unreached after a full-leg run is the
   genuinely untested set.
3. **Then enforce.** `scripts/reachability.sh --threshold N` gates; the landed
   pipeline is deliberately report-only until the baseline is honest.
