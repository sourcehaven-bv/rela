---
id: PLAN-LRI33T
type: planning-checklist
title: 'Planning: Pin extractions/setup-just to a commit SHA'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: pin both `extractions/setup-just` uses (archplan job in ci.yml,
archplan-weekly.yml) to the v3.1.0 commit SHA with a version comment. Out:
GitHub-owned actions (repo convention keeps tags), the other third-party actions
(goreleaser, go-test-coverage, markdownlint-cli2), Dependabot config.

**Acceptance Criteria:**
1. `grep -rn setup-just .github` shows only `@<40-char sha> # v3.1.0`.
2. The SHA equals `gh api repos/extractions/setup-just/git/ref/tags/v3.1.0`.
3. The Archplan job passes on the PR.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: two-line change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: no code)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: SHA pinning is GitHub's documented hardening practice)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** No action in the repo is SHA-pinned yet; third-party
actions use exact version tags. Dependabot's github-actions ecosystem is
configured in .github/dependabot.yml and updates SHA pins plus their version
comment by default.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** v3.1.0 is the latest v3.x tag. It is a lightweight tag,
so `git/ref/tags/v3.1.0` returns the commit
`f8a3cce218d9f83db3a2ecd90e41ac3de6cdfd9b` directly (commit message "This is
3.1.0"). Replace `@v3` with `@f8a3cce218d9f83db3a2ecd90e41ac3de6cdfd9b #
v3.1.0`. Alternative rejected: upgrading to v4 (out of scope; Dependabot will
propose it).

**Files to modify:** .github/workflows/ci.yml,
.github/workflows/archplan-weekly.yml.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no input handling)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no code)

**Input Sources & Validation:** The only external input is third-party action
code fetched at job time. Pinning to a commit SHA makes it immutable.

**Security-Sensitive Operations:** Supply chain: a moved tag could run attacker
code in the archplan job. The SHA pin removes that path.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: config-only change)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 by grep; AC2 by the gh api call; AC3 by the Archplan CI
job on the PR. actionlint on both files.

**Edge Cases:** Annotated tag would return a tag object, not the commit; checked
that v3.1.0 is lightweight (object type `commit`).

**Negative Tests:** N/A.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A wrong SHA fails the job at checkout; caught by CI on the PR.
Effort: xs.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: CI-only change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, no docs impact)

**Documentation Impact:** N/A - Internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: two-line config change; reviewed by rela-security-reviewer in the review phase)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
