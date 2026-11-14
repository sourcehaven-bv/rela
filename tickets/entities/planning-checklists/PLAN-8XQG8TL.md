---
id: PLAN-8XQG8TL
type: planning-checklist
title: 'Planning: Scheduled-tasks guide uses peer naming instead of remotes'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the peer naming in the scheduled-tasks guide source and its
generated copy. Out: any code change.

**Acceptance Criteria:**
1. `docs/scheduled-tasks.md` and its source name `remotes.yaml`, `remote:<name>` and link to `remote-sync.md`; checked by grep for "peer" (no hits) and by the link target existing.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: one-file docs fix)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: docs wording fix)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: docs wording fix)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A for this small change.

**Existing Solutions:** Names taken from `internal/remotesync/config.go`
(`FileName`) and `internal/scheduler/remotes.go` (`RemoteTaskPrefix`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] ~~Alternatives considered (document why rejected)~~ (N/A: the code is correct, only the text is wrong)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Edit the guide in `docs-project/`, then regenerate
`docs/` with `just docs`.

**Files to modify:** `docs-project/entities/guides/GUIDE-scheduled-tasks.md`,
`docs/scheduled-tasks.md` (generated).

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: documentation only)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: documentation only)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: documentation only)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: documentation only)

**Input Sources & Validation:** N/A.

**Security-Sensitive Operations:** None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: documentation only)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: documentation only)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: documentation only)

**Test Scenarios:** Grep both files for "peer"; confirm `docs/remote-sync.md`
exists; confirm `just docs` changes only the expected file.

**Edge Cases:** None.

**Negative Tests:** None.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: documentation only)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** None beyond editing the generated file directly, avoided by editing
the source.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, not an enhancement)

**Documentation Impact:**
- [x] docs/scheduled-tasks.md

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: wording-only docs fix)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** None.
