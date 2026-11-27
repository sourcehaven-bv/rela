---
id: PLAN-DZRIZJK
type: planning-checklist
title: 'Planning: plimsoll housekeeping: drop two dead directives, fix two stale prose references, document the store/Config exceptions and the #1452 ratchet bumps'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** The ticket's six items. In: drop the no-op `json:",inline"` tags,
extend the `Config` exception note, verify the directive and prose claims. Out:
any type decomposition.

**Acceptance Criteria:**
1. `just plimsoll` passes.
2. `go test ./internal/dataentryconfig/` passes, and the JSON output of the two
embedding structs is unchanged.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: xs chore)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: housekeeping)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: housekeeping)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Read plimsoll v0.2.0 `analyzer.limit`: an inline
directive overrides the default, so a directive below the default is a tighter
cap.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** See the ticket's Outcome section. The two sub-default
directives stay, because removing them loosens the cap.

**Files to modify:** `internal/dataentryconfig/config.go`,
`internal/dataentryconfig/theme.go`.

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: no behaviour change)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no behaviour change)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: no behaviour change)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no behaviour change)

**Input Sources & Validation:** N/A

**Security-Sensitive Operations:** None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: tag removal with no runtime effect)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: no behaviour change)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: no behaviour change)

**Test Scenarios:** plimsoll run; package tests; a throwaway program comparing
encoding/json output with and without the tag.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: no behaviour change)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A JSON decoder other than encoding/json reading the tag. None is used
in the package (json-iterator is only an indirect dependency).

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore ticket)

**Documentation Impact:** N/A, internal change.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: comment and no-op tag change; self-review only)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** None.
