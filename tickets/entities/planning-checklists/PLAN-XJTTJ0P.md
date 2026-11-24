---
id: PLAN-XJTTJ0P
type: planning-checklist
title: 'Planning: Calendar date buckets for grouped lists (day/week/month/year, near-then-far, newest first)'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: mapping form of `group_by.buckets` (by/for/then/order), SPA
sections, desc group sort, docs. Out: store null ordering, condition-based
groups (gap 10, other session).

**Acceptance Criteria:**
1. `{by: day, for: 7d, then: month}` gives overdue, one section per day, then months, then no date (useListGrouping.test.ts).
2. `{by: month, order: desc}` gives months newest first and reads the property descending (useListGrouping.test.ts, listParams.test.ts, TestNavItemsListQuery_GroupSortFollowsBucketOrder).
3. Invalid shapes fail config load (validate_groupby_test.go).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: extension of an existing option)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: @date-fns/tz already used)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: small extension)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** utils/dateBuckets.ts (calendar-day rule), records chart
week start (Monday).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `ListBuckets` type with scalar/mapping YAML and JSON;
per-shape validation; SPA `calendarBuckets.ts` keyed by day numbers; groupedSort
and navItemsListQuery sort descending for `order: desc`. Alternative of
server-side sections rejected: rows are already on the client, as for relative
buckets.

**Files to modify:** internal/dataentryconfig/groupby*.go, validate_groupby.go,
internal/dataentry/nav_items.go, frontend utils/calendarBuckets.ts,
composables/useListGrouping.ts, utils/listParams.ts, types/config.ts, i18n,
GUIDE-data-entry.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** operator config only; allowlisted units/orders;
`for` bounded 1..366.

**Security-Sensitive Operations:** none new. Sections are computed over rows the
list endpoint already gated and redacted; no new read or endpoint.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria.

**Edge Cases:** week across 2026/2027, DST night, invalid date 2026-02-31, empty
value, partial first month after window.

**Negative Tests:** unknown unit/order/key, `for` without d, 0d, 367d, `for` on
months, `then` without `for`, desc with `for`, labels the shape lacks.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** wire change of `buckets` (old cached SPA treats a mapping as
relative; harmless). Effort m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/data-entry.md (via GUIDE-data-entry) § Calendar
buckets.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-XPF5XYI, RR-3J4WHHX, RR-FI3QAF0, RR-WHUU8S2,
RR-HUHPPHJ, RR-OI3309O
