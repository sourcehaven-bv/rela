---
id: PLAN-UMTMEMV
type: planning-checklist
title: 'Planning: Drop stale realtimelimits allowance for channels in arch-lint'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: remove `realtimelimits` from `channels.mayDependOn` in
`.go-arch-lint.yml`. Out: the `PostBatch` rate limit (#2236); any change to
`channels.MaxBatch`.

**Acceptance Criteria:**
1. `.go-arch-lint.yml` no longer lets `channels` import `realtimelimits`; `just arch-lint` passes.
2. The godoc on `channels.MaxBatch` and the config agree.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: one-line config fix)
- [x] ~~Searched for existing libraries~~ (N/A: config fix)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: config fix)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: config fix)

**Research Doc:** N/A

**Existing Solutions:** `internal/realtime/batch_test.go` pins
`channels.MaxBatch` to `realtimelimits.MaxBatch`. Commit 1d2f320 removed the
import from `channels` for the archplan layer rule.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Delete the entry and its comment. Rejected: reword the
godoc instead, which would leave the linter allowing an import that archplan
forbids.

**Files to modify:** `.go-arch-lint.yml`

## Security Considerations

- [x] ~~Input sources identified~~ (N/A: lint config only)
- [x] ~~Input validation approach defined~~ (N/A: lint config only)
- [x] ~~Security-sensitive operations identified~~ (N/A: lint config only)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: lint config only)

**Input Sources & Validation:** N/A

**Security-Sensitive Operations:** N/A

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: lint config only)
- [x] ~~Negative test cases defined~~ (N/A: lint config only)
- [x] ~~Integration test approach defined~~ (N/A: arch-lint run is the check)

**Test Scenarios:**
1. `just arch-lint` passes.
2. `go build ./...` and `go test ./internal/channels/ ./internal/realtime/` pass.

**Edge Cases:** N/A

**Negative Tests:** N/A

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** None: `channels` has no import of `realtimelimits`, so arch-lint
stays green. Effort xs.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal lint config)
- [x] ~~Docs-checklist will be created~~ (N/A: chore, no docs impact)

**Documentation Impact:** N/A - Internal change, no user-facing docs needed

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: removes one stale lint entry)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** None
