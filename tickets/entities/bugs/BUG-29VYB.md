---
id: BUG-29VYB
type: bug
title: Skipped 'Rela Tickets' check disables auto-merge on Dependabot PRs
status: done
priority: medium
description: |
  Dependabot PRs that touch only files outside `tickets/entities/{bugs,features,tickets}`
  trigger the `Rela Tickets` job to be skipped via the job-level `if:` guard.
  GitHub's ruleset on `develop` lists `Rela Tickets` as a required status check.
  When the head commit's required check completes with conclusion `skipped`,
  GitHub disables auto-merge on the PR. The `dependabot-auto-merge` workflow
  successfully enables auto-merge after CI starts, but it is auto-disabled by
  GitHub a few minutes later when the skipped check is finalized — leaving the
  PR open requiring a manual merge.
why1: 'Required check `Rela Tickets` reported conclusion `skipped` on the head commit, and GitHub disables auto-merge when a required check ends in a non-success state.'
why2: 'The job uses a job-level `if:` guard (`!startsWith(github.head_ref, ''chore/'')` and `event_name == ''pull_request''`); when the guard is false the job is reported as skipped instead of running and reporting success.'
why3: 'The guard was written to short-circuit the job when ticket validation does not apply, without considering that GitHub treats a required check that resolves to `skipped` as failing for ruleset evaluation.'
why4: 'The interaction between job-level `if:` and `required_status_checks` rulesets is not documented in the workflow itself, and was only noticed when Dependabot PRs stopped auto-merging.'
why5: 'No convention in the repo for required-check jobs to always run and report success when their gate does not apply (the "skipped == failure" pitfall).'
prevention: |
  Required-check jobs must always execute and report `success`. Use a
  step-level decision step (`steps.gate.outputs.applies`) and gate downstream
  steps with `if:`, instead of a job-level `if:`. Document this pattern in
  CLAUDE.md for future workflow authors.
---

## Reproduction

1. Open a Dependabot PR that does not modify `tickets/entities/{bugs,features,tickets}/*.md`
   (e.g. https://github.com/sourcehaven-bv/rela/pull/594, an `actions/setup-go` bump).
2. The `dependabot-auto-merge` workflow approves and runs `gh pr merge --auto --squash`.
3. PR timeline shows `auto_squash_enabled` by `rela-coverage-bot[bot]` at T+0.
4. CI runs; `Rela Tickets` job is skipped via the `if:` guard, conclusion `skipped`.
5. ~5 minutes later the timeline shows `auto_merge_disabled` by `rela-coverage-bot[bot]`
   (GitHub attributes the disable to whoever enabled it).
6. PR remains open with `mergeStateStatus: BLOCKED`, `auto_merge: null`.

## Root cause

The ruleset on `develop` lists `Rela Tickets` in `required_status_checks`. GitHub's
auto-merge feature disables itself when any required check on the head commit ends
in a non-success conclusion, including `skipped`. The job-level `if:` guard makes the
job skip cleanly from a workflow perspective, but the resulting check-run reports
`conclusion: skipped` against the head commit, which the ruleset evaluator treats
as failing.

## Fix

1. **Workflow change**: remove the job-level `if:` on `rela-tickets`. Add a first
   step `Decide whether ticket gate applies` that sets
   `steps.gate.outputs.applies`, and gate all subsequent steps with
   `if: steps.gate.outputs.applies == 'true'`. The job always runs and always
   reports `success`; when the gate does not apply, only the decision step runs.
2. **Dependabot rework** (so the auto-merge path stays clean and so we drop a
   workaround that hid this bug for trusted GitHub Actions while leaving npm/Go
   stuck): move the supply-chain soak from PR-age (the deleted
   `dependabot-deferred-merge.yml` cron) to publication-age via
   `cooldown:` in `.github/dependabot.yml`. Simplify
   `dependabot-auto-merge.yml` to a single immediate `gh pr merge --auto --squash`
   for every Dependabot PR. Cooldown values: 7d default for npm (14d major),
   3d for Go modules (7d major), 7d for third-party Actions (trusted-org Actions
   excluded).
