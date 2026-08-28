---
id: BUG-CI7XKP
type: bug
title: "Stacked PRs ran zero CI checks, and an empty check list read as green"
kind: bug
priority: high
effort: s
tags:
    - ci
    - security
status: done
description: >-
  ci.yml and codeql.yml filtered the pull_request trigger on branches
  [main, develop]. That filter matches the TARGET branch, so a PR stacked on a
  feature branch matched no workflow and ran zero checks — no SAST, no SCA, no
  tests — while its check list looked clean because it was empty. Fixed by
  removing the branches: filter from the PR trigger in both workflows.
why1: "ci.yml and codeql.yml filtered pull_request on branches [main, develop]. That filter matches the TARGET branch, so a PR opened against a feature branch matched no workflow and ran nothing."
why2: "The filter was written when every PR targeted develop. Stacked PRs were introduced later without revisiting the trigger, and nothing failed loudly when they did — the absence of a run produces no signal."
why3: "An empty check list is indistinguishable from a passing one on the PR page and in `gh pr checks`, which reports success when nothing failed. Absence of evidence rendered as evidence of absence."
why4: "The verification I ran asserted the absence of failures rather than the presence of the required jobs, so it confirmed the same non-signal the PR page showed."
why5: "There was no control asserting that the mandated security scans (SAST/SCA) actually executed for a change. Required-checks enforcement is a branch-protection concern that no repo-level test covers, so a workflow that never triggers is invisible to every other guardrail."
prevention: "Removed the branches: filter from pull_request in ci.yml and codeql.yml, so every PR is scanned regardless of target. Recorded in AM-ci-runs-on-every-pr. Verification of CI must check that named required jobs are PRESENT, not merely that none failed."
---

## Summary

PR #1343 added a new authenticated HTTP endpoint (remote MCP) and **no CI
workflow ran on it** — no CodeQL, no Vulnerability Check, no tests. This was
caught in review (IB-review, finding 1, POLICY-015 §4), not by any automated
control.

Only `Dependabot Auto-Merge` triggered, and it reported `skipping`.

## Root cause

`ci.yml` and `codeql.yml` both declared:

```yaml
pull_request:
  branches: [main, develop]
```

The `branches:` filter on `pull_request` matches the **target** branch. PR
#1343 targeted `fix-govulncheck` (it was stacked on PR #1338), so it matched
neither workflow and no run was created.

## Why it wasn't noticed

An empty check list looks exactly like a passing one:

- The PR page shows no red.
- `gh pr checks` returns success when nothing has failed.

My own verification made this worse rather than catching it. The completion
guard was `length>0 and all(.bucket!="pending")`, which a single skipped
`auto-merge` entry satisfies, and the failure filter was
`select(.bucket=="fail")`, which is empty when nothing ran. So the monitor
printed "ALL GREEN" twice for a branch that had never been built. **The check
asserted the absence of failure; it should have asserted the presence of the
required jobs.**

## Fix

Removed the `branches:` filter from `pull_request` in both workflows. Push
triggers keep their filters (there is no value in building every pushed
branch); PR triggers now fire regardless of target, which is the case that
matters — a PR is a request to merge code, whatever it is stacked on.

## Impact

Any stacked PR merged before this fix reached `develop` without SAST/SCA
coverage on its own branch. The code was scanned once it landed on `develop`
(that trigger was never broken), so this is a gap in pre-merge assurance
rather than unscanned code in production.

Affected in this arc: #1336 (targeted `fix-govulncheck`) and #1343. #1338
targeted `develop` and was scanned normally.
