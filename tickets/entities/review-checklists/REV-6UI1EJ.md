---
id: REV-6UI1EJ
type: review-checklist
title: 'Review: CalVer releases (vYY.M.BUILD) with an automated tag cutter'
status: done
---

## Automated Checks

- [x] All tests pass — CI `Test` job passed; no Go code changed
- [x] Lint clean — CI `Lint`, `Lint Markdown`, `Architecture`, `God-object lint`
and CodeQL `Analyze (actions)` all passed
- [x] Coverage maintained — no Go code changed, so package floors are unaffected

CI on PR #1202: all jobs green except two, neither caused by this change —

- **Postgres Backend** — infrastructure flake: `docker pull postgres:16` hit
`registry-1.docker.io` timeouts through all three retries. No code path in this
PR touches postgres.
- **Rela Tickets** — the policy gate that requires every PR to carry a ticket
entity. Resolved by this ticket and its checklists.

## Code Review

- [x] ~~Run `/code-review`~~ (N/A: no application code — a shell script, two
workflow files, and docs; reviewed directly against the actions-injection
guidance instead)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes — `release.yml` is
deliberately a 2-hunk diff (one comment + the `workflow_dispatch` trigger) with
no logic changes

**Review Responses:** none

Security note: every `workflow_dispatch` input (`ref`, `alpha`, `dry_run`)
reaches `run:` blocks through `env:` bindings rather than `${{ }}`
interpolation, per the workflow-injection guidance.

## Acceptance Verification

- [x] Each acceptance criterion tested — see the evidence block in IMPL-BI76XJ
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Tag computed from the date, monthly counter — PASS (`v26.7.0` → `.1` → `.2`)
- Same-month repeats do not collide — PASS (including across `--alpha`)
- Prerelease supported — PASS (`v26.7.2-alpha`, and GoReleaser's
`prerelease: auto` already keys off the `-` in the tag)
- Works with the existing GoReleaser pipeline unchanged — PASS (valid semver,
verified against `Masterminds/semver`)
- Works with every installer format unchanged — PASS (MSI limits verified; nfpm
round-trips the version)
- Release triggers after the workflow pushes the tag — PASS by construction: the
push uses a GitHub App token, which fires `on: push`; `security.yml` already
pushes branches with this same app, so the `contents: write` permission is
established. Cannot be executed end-to-end until merged to the default branch.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (`docs/releasing.md`)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-NX83KB

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — cutting a release is a
`workflow_dispatch` from the Actions tab, with `dry_run` to preview the next tag
first

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — modulo the two failures analysed above, one an
infra flake and one resolved by this ticket
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1202
