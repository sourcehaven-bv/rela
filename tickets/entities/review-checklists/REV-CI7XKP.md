---
id: REV-CI7XKP
type: review-checklist
title: 'Review: Stacked PRs ran zero CI checks, and an empty check list read as green'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full `./internal/...` + `./cmd/...` suite green
- [x] Lint clean (`just lint`) — `golangci-lint` 0 issues; workflow YAML re-parsed with a loader
- [x] Coverage maintained (`just coverage-check`) — no Go code changed by this fix

The fix itself is two `on.pull_request` trigger changes. Its real verification
is the one the bug is about: **the required jobs now appear**. Confirmed by
re-targeting PR #1343 to `develop` and checking the check list contains the
security scans by name, not merely that nothing failed.

## Code Review

- [x] Run `/code-review` command — externally reviewed (IB-review, CISO) on PR #1343; this bug IS finding 1 of that review
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — the single finding (Matig / moderate) is fixed here
- [x] Self-reviewed the diff for unrelated changes — the diff is 2 trigger blocks plus their explanatory comments

**Origin:** the finding was raised against POLICY-015 §4 ("Geautomatiseerde
beveiligingsscans zijn onderdeel van de CI/CD-pipeline"). The reviewer was
right and my own reporting was wrong: I had told the user "ALL GREEN" twice for
a branch that never built.

## Verification

- [x] Acceptance criteria met — every PR now triggers CI and CodeQL regardless of target branch
- [x] Manual testing performed — `gh run list --branch tkt-bdg8u9-remote-mcp-http` showed only 3 skipped `Dependabot Auto-Merge` runs before the fix
- [x] No regressions introduced — push triggers keep their `branches:` filters, so no change to what gets built outside PRs
- [x] Documentation updated — `AM-ci-runs-on-every-pr` records the measure, including a copy-pasteable presence check

## Lessons captured

The bug's why4 is about my verification method, not the workflow: I asserted
the **absence of failures** where I should have asserted the **presence of
required jobs**. `jq 'select(.bucket=="fail")'` returns empty both when
everything passed and when nothing ran. `AM-ci-runs-on-every-pr` carries the
corrected check so the next person does not repeat it.
