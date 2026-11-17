---
id: PLAN-ET76RY1
type: planning-checklist
title: 'Planning: Rename dataentrywire remotes adapter to remotes_wiring.go'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In scope: rename `internal/dataentrywire/remotes.go` and
`remotes_test.go` to `remotes_wiring.go` and `remotes_wiring_test.go`, and
update references to the old path. Out of scope: code changes, the
`remote_mcp_*_test.go` files, and the `.archplan/` bookkeeping.

**Acceptance Criteria:**

1. The two files carry the new names and their content is unchanged: `git
diff -M --stat` shows two 100% renames.
2. The module builds and the package tests pass: `go build ./...` and `go test
./internal/dataentrywire/...`.
3. Package boundaries still hold: `just arch-lint`.
4. Nothing refers to the old path: `git grep 'dataentrywire/remotes.go'` is
empty.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: file rename)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: file rename)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: file rename)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: file rename)

**Research Doc:** N/A for a small change.

**Existing Solutions:** The `*_wiring.go` suffix is already used for wiring
files, for example `internal/cli/mcp_wiring_fs.go`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] ~~Dependencies identified (packages, APIs, types)~~ (N/A: no code changes)

**Technical Approach:** `git mv` both files. The file adapts the scheduler to
the `dataentry.RemoteService` port, so it is wiring. Alternative: move the
adapter into another package. Rejected, because the rename states the role
without changing code.

**Files to modify:**

- `internal/dataentrywire/remotes.go` → `remotes_wiring.go`
- `internal/dataentrywire/remotes_test.go` → `remotes_wiring_test.go`

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: no code changes)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no code changes)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: no code changes)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no code changes)

**Input Sources & Validation:** None; no code changes.

**Security-Sensitive Operations:** None; no code changes.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: no behaviour changes)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: no behaviour changes)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: no behaviour changes)

**Test Scenarios:** Each acceptance criterion names its own command. The
existing tests in the renamed test file keep covering the adapter.

**Edge Cases:** None; no behaviour changes.

**Negative Tests:** None; no behaviour changes.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: no code changes)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A tool or document may name the old path. Mitigation: grep the
repository, including docs, rules and `.go-arch-lint.yml`. Effort: xs.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal rename)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, not an enhancement)

**Documentation Impact:** N/A: internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: file rename with no design content)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** None.
