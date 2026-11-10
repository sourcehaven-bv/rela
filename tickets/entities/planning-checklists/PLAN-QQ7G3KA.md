---
id: PLAN-QQ7G3KA
type: planning-checklist
title: 'Planning: Fix flows M2 documentation findings'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the PLAN-MFSBIYB body (run list route, completed-step answer,
threat-model DoS row), the `complete_flow_step` MCP tool description,
GUIDE-mcp-server and the generated `docs/mcp-server.md`. Out: behavior changes,
the `SubjectRuns` cap itself (RR-6KWVWY8 on TKT-FWQZGAM), and M3 (TKT-XI7IDBS).

**Acceptance Criteria:**

1. PLAN-MFSBIYB names `HolderLookup.Headers` for the run list; checked by
reading `FlowControl.RunList`.
2. PLAN-MFSBIYB lists `{written, moved, state, finished, problem}`; checked
against `flowview.Completed`.
3. The tool description and the MCP guide name `problem`; checked against
`flowview.Completed.Problem`.
4. The DoS row says truncation with `truncated: true` and that `SubjectRuns`
has no cap; checked against `MaxTaskCandidates`, `MaxRunList` and `SubjectRuns`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: documentation fix)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: documentation fix)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: documentation fix)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: documentation fix)

**Research Doc:** N/A

**Existing Solutions:** N/A

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] ~~Alternatives considered (document why rejected)~~ (N/A: text correction)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Rewrite each wrong sentence to match the code on
develop, then regenerate `docs/` with `just docs`.

**Files to modify:** `tickets/entities/planning-checklists/PLAN-MFSBIYB.md`,
`internal/mcp/tools_flows.go`,
`docs-project/entities/guides/GUIDE-mcp-server.md`, `docs/mcp-server.md`.

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: no code path changes)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no code path changes)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: no code path changes)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no code path changes)

**Input Sources & Validation:** N/A

**Security-Sensitive Operations:** N/A

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: documentation fix)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: documentation fix)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: documentation fix)

**Test Scenarios:** Each criterion is checked by reading the named code.
Existing MCP tests cover the tool registration.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: no code path changes)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** None beyond wording; effort xs.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore ticket)

**Documentation Impact:** GUIDE-mcp-server and `docs/mcp-server.md`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: documentation fix)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** N/A
