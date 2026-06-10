---
id: TKT-EXL94Y
type: ticket
title: Default package coverage floor so new untested packages fail CI
kind: chore
priority: medium
effort: xs
status: done
---

## Problem

`.testcoverage.yml` declares its purpose as "floors exist to catch new untested
packages added and core packages silently losing tests" — but
`threshold.package: 0` means only the 11 explicitly-overridden packages and the
65% total are enforced. A brand-new package with zero tests sails through CI;
the total threshold dilutes too slowly to catch it.

## Approach (agreed with reviewer in session)

1. Measure current per-package coverage on develop.
2. Set the default package floor to 50.
3. Add explicit lower overrides for the known sub-50 packages, set ~5pp below current (the file's existing convention) so nothing currently passing starts failing.
4. Exclude genuine helper packages (e.g. entitymanagertest) where coverage is meaningless.
5. Verify `just coverage-check` green locally.

Per session decision: separate PR (2 of 3 split from the original combined
CI-quality task).

## Verification

- `just coverage-check` green on the branch.
- Negative check: a scratch package with an untested function fails the check (verified locally, then removed).
