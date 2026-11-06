---
id: PLAN-3LND3U8
type: planning-checklist
title: 'Planning: Make ygo upstream releases and advisories visible to the ygows fork'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: version const in the fork, drift test against go.mod, daily OSV
job with issue filing, docs. Out: a Postgres cluster relay; auto-re-syncing the
fork.

**Acceptance Criteria:** A ygo bump in go.mod fails `go test
./third_party/ygows/`. An OSV advisory on a forked package at UpstreamVersion
opens a `security` issue; a clean run closes it.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change, shape given in issue #2105)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows security.yml's own pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** security.yml issue filing and
govulncheck-fixable-test.sh; OSV /v1/query API.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `UpstreamVersion` const; `TestUpstreamVersion` parses
go.mod; `scripts/ygows-advisories.sh` filters OSV responses by import path; job
in security.yml. Rejected: osv-scanner on a synthetic lockfile (no import-path
filter); x/mod/modfile (new dependency for a test).

**Files to modify:** third_party/ygows, scripts/, security.yml, ci.yml,
justfile, GUIDE-server-security.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** OSV API JSON (parsed by jq, values passed as
data, never evaluated). Issue dedup filters on author like security.yml.

**Security-Sensitive Operations:** Writes issues with GITHUB_TOKEN; advisories
are public.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** TestUpstreamVersion on the real go.mod; TestRequiredVersion
table; ygows-advisories-test.sh in CI.

**Edge Cases:** replace directives, require blocks, paging, withdrawn, ignored,
prefix-only path match, GHSA without paths.

**Negative Tests:** bad JSON exits 2; missing module errors.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** GHSA records without paths cause false positives; mitigated by
osv-ignore.txt.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** third_party/ygows/README.md and GUIDE-server-security.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design set by the CISO issue #2105; reviewed in /code-review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** none
