---
id: TKT-L9LV0E
type: ticket
title: 'Reachability floor: classify the unreached set with reasoned coverage-ignore directives'
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

TKT-DWO4ZB landed the reachability *pipeline* (`scripts/reachability.sh`, a
`just reachability` recipe, a report-only CI job, `scupper@v0.2.0` pinned with
`-d coverage-ignore --require-reason`) but deliberately left the *classification*
open: it shipped 41 directives, enough to prove the mechanism, not to describe
the codebase.

This ticket supplies the classification layer: 260 additional reasoned
`// coverage-ignore*` directives across 109 Go files, each naming a category and
a per-case justification, plus a regenerated `COVERAGE-HONESTY-REPORT.md`.

## Scope

1. **Classify.** Every directive states a category (`defensive`, `os-fs-event`,
   `main-or-wiring`, `unreachable`, `unreachable-default`, `panic-invariant`,
   `invariant`, `external-tool`, `postgres-only`, `os-fs`, `sealing`) and a
   reason specific to the call site. `--require-reason` makes an unexplained
   dismissal a hard error, so each one stays reviewable in the diff.
2. **Use the landed dialect.** The classification was originally authored as
   `//scupper:ignore`, scupper's own default spelling. The pipeline that landed
   invokes `-d coverage-ignore`, which reads only `// coverage-ignore*:`
   comments, so the original spelling would have been silently inert. All
   directives are converted, preserving each reason verbatim. This follows the
   decision recorded in TKT-DWO4ZB: one annotation vocabulary, shared with
   `go-test-coverage`, not a second tool-specific dialect.
3. **Regenerate the audit.** `COVERAGE-HONESTY-REPORT.md` is rebuilt from a
   current pipeline run against `origin/develop`, replacing a 2026-08-28
   snapshot whose every headline figure was stale.

## Measured result

| Metric | Value |
| --- | --- |
| Statements measured | 58,326 |
| Reached | 46,157 (79.1%) |
| Dismissed via directives | 1,187 |
| Baseline on `develop`, same profile | 78.6%, 395 dismissed |

Still report-only. No threshold is enforced: the e2e and postgres legs were not
run here, so pgstore (6.8%), dataentry (82.9%) and rela-server (10.0%) are
understated by the measurement, not by the code. Gating on today's partial
number would encode that gap as if it were a property of the source.

## Validation performed

Each directive was checked against the merged profile rather than taken on
trust:

- 106 single-line dismissals: none sits on a statement the profile reports as
  reached (column-aware — a dismissal on an `if` line dismisses the body, not
  the condition).
- 161 `-start`/`-end` blocks: 112 fully unreached, 27 partially reached, 19
  contain no statements.
- **3 blocks were fully reached and were dropped as stale** — the code they
  called unreachable now runs under test (`internal/config/config.go`,
  `internal/predicate/eval.go`, `internal/store/fsstore/watcher.go`).

## Out of scope

Raising a threshold, and writing tests for the ~12,169 genuinely unreached
statements. The report's closing section records the order: measure all legs,
re-classify what remains, then enforce.
