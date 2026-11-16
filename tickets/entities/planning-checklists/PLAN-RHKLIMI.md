---
id: PLAN-RHKLIMI
type: planning-checklist
title: 'Planning: Run the full race suites after merge, not in the merge queue'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: drop `-race` from the Test job's suite and the Postgres Backend
suites on `pull_request` and `merge_group`; targeted race steps; full race on
push; an issue on post-merge failure. Out: merge-queue ruleset settings, path
skips (dropped, see ticket).

**Acceptance Criteria:**
1. On a PR, the Test job runs `go test` without `-race` and a second step runs
the concurrency tests under `-race`. Checked on this PR's CI run.
2. On push to `develop`, the Test and Postgres suites run under `-race`
(`GOFLAGS=-race`). Checked on the run after merge.
3. A failed push run files one issue and comments on it later. Checked by
reading the job; it reuses the `security.yml` pattern.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small CI change)
- [x] ~~Searched for existing libraries~~ (N/A: workflow config only)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: standard GitHub Actions)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** The issue filing copies
`.github/workflows/security.yml`. The skip-on-detection idea was compared with
the ClamAV job's step.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** A step-level `GOFLAGS` expression adds `-race` on push.
Targeted steps find tests by name with grep. `post-merge-alert` needs `test` and
`postgres`.

Rejected: nightly runs (a failure spans many merges); a separate race job
(duplicates the setup and needs a ruleset change to gate).

**Files to modify:** `.github/workflows/ci.yml`, `CLAUDE.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Expressions use only `github.event_name`,
`github.ref_name`, `github.sha` and job results, passed through `env:`.

**Security-Sensitive Operations:** `post-merge-alert` gets `issues: write` and
runs only on push, never on PR code from forks.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined~~ (N/A: verified by the PR's own CI run)

**Test Scenarios:** See acceptance criteria. The targeted race set was run
locally: 84 tests in 34 packages, 3 minutes, green.

**Edge Cases:** If the grep finds no packages, `go test` gets no package
arguments and fails, rather than passing silently.

**Negative Tests:** actionlint reports no new findings.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A data race outside a named concurrency test reaches `develop`.
Mitigation: the push run catches it per commit and files an issue.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal CI change)
- [x] ~~Docs-checklist will be created~~ (N/A: chore, not an enhancement)

**Documentation Impact:** CLAUDE.md race rule updated.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: CI config change; approach agreed with Jeroen in session)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
