---
id: PLAN-ORZ4LKT
type: planning-checklist
title: 'Planning: ACL guide: adopted row on a push remote is deleted when gone'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the one sentence about `on_gone: delete` on a push remote in
GUIDE-acl-security.md and its generated copy. Out: code changes; the Remotes
page issue (#2230).

**Acceptance Criteria:**
1. The ACL guide states that an adopted, unedited row on a push remote is deleted like any other linked row, and that only a row edited here since the last sync follows the conflict rule. Checked by reading `internal/remotesync/gone.go` (`goneEntity`, `goneRelation`) against the new text.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: one-sentence docs fix)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: docs fix)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: docs fix)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** GUIDE-remote-sync.md already describes the behaviour
correctly; the new wording follows it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] ~~Dependencies identified (packages, APIs, types)~~ (N/A: docs only)

**Technical Approach:** Edit the sentence in docs-project and regenerate docs
with `just docs`. Changing the code to hold adopted rows on push remotes was
rejected: the current behaviour is intended and pinned by
`TestPush_PushedRowIsDeletedWhenGone`.

**Files to modify:** docs-project/entities/guides/GUIDE-acl-security.md,
docs/acl-security.md (generated).

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: docs only)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: docs only)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: docs only)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: docs only)

**Input Sources & Validation:** N/A

**Security-Sensitive Operations:** N/A

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: docs only)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: docs only)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: docs only)

**Test Scenarios:** Manual comparison of the text with gone.go; `just docs`
regenerates with no diff beyond the sentence.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: docs only)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** None beyond wording; effort xs.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/acl-security.md only.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: one-sentence docs fix)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review for a docs fix)

**Design Review Findings:** none
