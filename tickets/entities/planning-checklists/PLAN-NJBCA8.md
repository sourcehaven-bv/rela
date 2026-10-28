---
id: PLAN-NJBCA8
type: planning-checklist
title: 'Planning: Record the archplan baseline and run archplan in CI'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: archplan version pin, two tuning fixes (comment gate guard
markers, `gated_by` for rollup), `.archplan/baseline.json` with per-group
reasons, an `Archplan` PR job in ci.yml, a weekly workflow for the trace-based
checks. Out: fixing the accepted debt findings; the SQLite FTS shadow-table
regression in archplan (upstream).

**Acceptance Criteria:**
1. `archplan ci --base origin/develop` passes on the PR (the Archplan job).
2. Every baseline entry has a group reason; none says "baseline".
3. `handleV1Comments` is no longer `unchecked_entry`; rollup is no longer
`unguarded_path`.
4. The weekly workflow updates one issue instead of opening duplicates.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the baseline proposal in .ignored/baseline-proposal.md covers the analysis)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: archplan's own `ci` command is the reference)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** archplan `ci`, `check --tighten` and `Findings.decide`
record the baseline. security.yml and security-scan.yml are the models for a
scheduled job that files one tracking issue. Existing guard markers:
detailactions.go, handlers_git.go. Existing `gated_by`: tracer, search,
validator GLOSSARY.md.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Pin v26.10.30: v26.10.29 lacks the misplaced_file and
unrelated_parts detectors that the proposal reviewed, so its finding set
differs. Re-run the routes suite, record reasons per detector with a script over
`archplan.findings.Findings.decide`, then `archplan check --tighten` for the
ratchet. The PR job reads the archplan version from the justfile. Alternative
rejected: `check --update-baseline` (generic reason).

**Files to modify:** justfile, internal/dataentry/comments_ops.go,
internal/rollup/GLOSSARY.md, .archplan/baseline.json, .github/workflows/ci.yml,
.github/workflows/archplan-weekly.yml.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Workflow inputs: base branch from the event (env
var, not spliced), the archplan version parsed from the justfile with a strict
pattern, the issue title passed to jq as `--arg`.

**Security-Sensitive Operations:** `ARCHPLAN_CI_TRUST=1` runs the export
commands from .archplan/config.json on `pull_request` (not
`pull_request_target`), so a fork PR runs with a read-only token. The repo is
public, so the weekly issue carries counts and finding ids only; details go to
code scanning via SARIF.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. Local `archplan ci --base origin/develop` without traces or hotspots, as
in CI. Then the Archplan job on the PR itself.
2. Count reasons in baseline.json.
3. `archplan check --all --json` after the tuning fixes.
4. actionlint on both workflows; a manual `workflow_dispatch` after merge.

**Edge Cases:** Base export on this PR uses develop's older exporter pin;
merge_group has no `base_ref`, so the base comes from `merge_group.base_ref`.

**Negative Tests:** archplan exit 2 (export failure) fails the weekly run
instead of being reported as findings.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Finding ids change between archplan versions: the pin is shared by
the justfile and both workflows. The weekly routes suite may be flaky on a
runner: it does not block PRs.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: docs-project has no developer tooling docs; the workflows carry header comments)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, no user-facing change)

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the approach was reviewed and approved by the user as the baseline proposal)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A
