---
id: PLAN-DT0AB8O
type: planning-checklist
title: 'Planning: Run E2E and Frontend CI jobs in the Playwright image'
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the Frontend job in `.github/workflows/ci.yml` moves to
Playwright's image; E2E and every other apt-get step in ci.yml get retries. Out:
other workflow files, other jobs' structure.

**Acceptance Criteria:**
1. Frontend passes on the PR with no apt step and no `--with-deps`.
2. The image tag matches the Playwright version in the frontend lockfile.
3. E2E and the other apt jobs pass with the retries in place.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small CI change)
- [x] Searched for existing libraries that solve this problem (Playwright's official CI image)
- [x] Checked codebase for similar patterns or reusable code (the Semgrep job already uses `container:`)
- [x] Looked for reference implementations in other projects (Playwright's GitHub Actions docs: container with `--user 1001`)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: CI-only change)

**Research Doc:** N/A

**Existing Solutions:** Playwright's documented container setup.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Frontend gets `container:` with
`mcr.microsoft.com/playwright:v1.64.0-noble` pinned by digest, `--user 1001`,
and loses its browser install step. E2E stays on the runner because its
bubblewrap sandbox needs namespaces a default container blocks; its bubblewrap
and `--with-deps` installs are retried three times. Every `apt-get update` in
ci.yml is retried three times and apt-get gets `-o Acquire::Retries=5`.
Alternative rejected: E2E in the container, which needed `--privileged` and a
downloaded bubblewrap .deb.

**Files to modify:** `.github/workflows/ci.yml`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: CI config, no secrets used by these jobs)

**Input Sources & Validation:** Threat model (supply chain): the only new
third-party artifact is `mcr.microsoft.com/playwright`, Microsoft's official
image, pinned by exact version tag matching the lockfile and by digest, so a
retagged image cannot change what runs. The job gets no secrets.

**Security-Sensitive Operations:** None; the retries re-run the same apt and
Playwright installs from the same sources.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined~~ (N/A: CI runs on the PR are the test)

**Test Scenarios:** CI on the PR runs Frontend in the container and E2E on the
runner with retries.

**Edge Cases:** All three attempts fail: the step exits non-zero.

**Negative Tests:** A wrong image digest fails the container pull.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** The image tag and digest must be bumped with Playwright in the
frontend lockfile. A long mirror outage still fails the apt jobs after three
attempts.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: CI-only change)
- [x] ~~Docs-checklist will be created~~ (N/A: chore, no user-facing docs)

**Documentation Impact:** N/A

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small CI-only change, approach approved by the user)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
