---
id: TKT-PCLGGL
type: ticket
title: Weekly fuzz sweep over all targets with auto-filed issues
kind: test
priority: medium
effort: s
status: done
---

## Problem

The repo has 39 fuzz targets — including the differential store fuzzer and 18
cross-backend conformance targets — but the per-PR CI fuzz job runs 3 of them
for 10s each. The other 36 only execute their seed corpora as regular tests; the
best bug-finding machinery in the repo gets no fuzz time.

## Approach (agreed with reviewer in session)

1. `scripts/fuzz-all.sh`: discovers `Fuzz*` targets by scanning test files (new targets are swept automatically — no stale hand-list), runs each for `$FUZZTIME` (default 25s), collects failures into `fuzz-failures.txt`, exits non-zero if any failed. pgstore targets self-skip without `RELA_TEST_DATABASE_URL` (postgres-service wiring is a noted follow-up).
2. `.github/workflows/fuzz-sweep.yml`: weekly cron (Mondays 06:00 UTC, before the security scan) + `workflow_dispatch`; on failure uploads the failing corpus inputs as an artifact and **auto-files a GitHub issue** (label `fuzz-failure`) with the failed targets, run link, and reproduction instructions — deduped by commenting on an existing open `fuzz-failure` issue instead of creating a new one each week.
3. `just fuzz-all` recipe for local runs.
4. The per-PR fuzz job stays as-is (fast smoke).

Per session decision: separate PR (one of three split out of the original
combined task 4).

## Verification

- Script run locally with a short FUZZTIME completes across all discovered targets and reports failures correctly (verified by injecting a deliberate failure).
- Workflow lint-clean (actionlint via CI's Analyze (actions) job); CI green on PR.
