---
id: PLAN-VIDXKJJ
type: planning-checklist
title: 'Planning: List buckets: relative_weeks with this week and next week'
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** a new `buckets: relative_weeks` scheme (Go config + SPA + docs). Not
in scope: changing `relative`, or the Atlas config (follows after release).

**Acceptance Criteria:**
<!-- Each criterion must have a concrete test scenario -->
1. Wed 25 Nov: Fri 27 and Sun 29 are `this_week`, Mon 30 and Sun 6 Dec `next_week`, Mon 7 Dec `later`.
2. On a Sunday there is no `this_week`; Monday is `tomorrow`.
3. `labels` accepts `this_week` and `next_week` for the scheme and rejects `next_7_days`.
4. Empty buckets are not shown.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change, no research doc)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: date arithmetic on calendar days, no library needed)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: extends the existing relative scheme)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: read the date-buckets section of GUIDE-data-entry, no concept entity consulted)

**Research Doc:** N/A

**Existing Solutions:**
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** new scalar scheme `relative_weeks` next to `relative`;
`bucketOf` takes a `weeks` flag and uses the Monday-based weekday of today; a
second bucket order. Alternative rejected: calendar buckets (`by: week`) give
"Week of 30 Nov" titles and cannot be relabelled.

**Files to modify:**
`internal/dataentryconfig/{groupby_buckets,validate_groupby}.go`,
`frontend/src/{utils/dateBuckets,composables/useListGrouping,types/config}.ts`,
i18n en/nl, GUIDE-data-entry.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** only the `buckets:` key of data-entry.yaml,
validated against an allowlist at load; an unknown scheme or label key is a
config error.

**Security-Sensitive Operations:** none.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** `dateBuckets.test.ts` and `useListGrouping.test.ts` for 1, 2
and 4; `validate_groupby_test.go` for 3.

**Edge Cases:**
<!-- List specific edge cases and expected behavior. Consider:
- Empty/null/missing values
- Boundary values (0, -1, MAX_INT)
- Special characters, unicode, null bytes
- Concurrent access
- Resource exhaustion
-->

**Negative Tests:** `next_7_days` label under `relative_weeks` fails validation.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** week boundary and time zone; the bucket uses the reader's calendar
day like the other buckets. Effort: s.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] docs/metamodel.md - New metamodel features
- [x] docs/cli-reference.md - New/changed commands
- [x] docs/data-entry.md - UI changes
- [x] CLAUDE.md - New patterns or conventions
- [x] README.md - Project-level changes
- [x] N/A - Internal change, no user-facing docs needed
-->

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: not run, small change to one scheme)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review was run)

**Design Review Findings:** none (no design review run).
