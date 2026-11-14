---
id: PLAN-PWSILO6
type: planning-checklist
title: 'Planning: Page the data-entry collection reads'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: an id narrowing in `scopedHeaders` (`scopeRequest.ByIDs`) used by the list
Go path for the page scope and the `?q=` hits; `visibleListByTypes`' wildcard
branch reads headers per type through `scopedHeaders`; the command list context
and the settings relation targets read through the gated header read;
`listFromStoreByTypes` and `listAllFromStore` are deleted; their allowlist
entries are removed; the page-scoped, incoming-ordered and `?q=` budget tests
assert rows.

Out (follow-ups): list `condition:` and non-lowering scopes (TKT-I39Y7N4);
`executeQuery`, dashboard, nav status and the whole-set callers (TKT-D3RAB0E,
overlapping TKT-AIEGHU, TKT-8AUD1U, TKT-CKLZMWB, TKT-M58EQ1W); the settings
search picker (TKT-WJP6PBM).

**Acceptance Criteria:**
1. A page-scoped list and a `?q=` list read the same rows over a store of 100
and of 1,000 padding entities (budget tests).
2. Reading by ids returns exactly the full read restricted to those ids, on
both verdict branches, and no ids reads nothing
(`TestScopedHeaders_ByIDsIsTheFullReadRestricted`).
3. A list-context command payload is row-gated and redacted
(`TestCommandExec_ListContextIsGatedAndRedacted`).
4. `TestNoNewUnpagedReads` passes with the three helpers entries removed.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the ticket names the approach)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal store API)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal store API)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `store.MatchingFaces` answers the stamped query for
candidate ids (used by `applyScope` and the ACL row gate); `loadHitHeaders`
(queryservice.go) reads headers by face. `headersByIDs` combines the two.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `scopedHeaders` builds the verdict query once, stamps
world and faces, then either reads by ids (MatchingFaces + one header read per
face), scans the type, or drains the GraphQuery. The list Go path checks
`readWithheld` before the `?q=` search (RR-X56H), then passes page-scope ids and
hits as the narrowing. Alternative rejected: intersecting after the full read
(the old shape). Paging `executeQuery` was left to TKT-D3RAB0E because its
callers need totals and Go-side aggregation that existing tickets own.

**Files to modify:** internal/dataentry/scopedread.go, api_v1.go, helpers.go,
commands.go, settings_handlers.go, budget tests, internal/archguard allowlist,
.claude/rules/collection-reads.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `?q=` text and the page-scope params, both
already validated; the ids they yield only narrow the gated query.

**Security-Sensitive Operations:** The ACL verdict, world and faces are stamped
on the id read exactly as on the scan. The command list payload was ungated and
unredacted; it now goes through the gated read and field redaction.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See the acceptance criteria; the budget tests run the full
HTTP list handler.

**Edge Cases:** No ids; ids of another type; unknown ids; ids the verdict or
props exclude.

**Negative Tests:** A principal with no ticket read gets an empty command
payload.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Ordering of rows read by ids differs from the scan: mitigated by
sorting by id then face, the scan's order. Effort: l (ticket), m for this part.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal read path; the command list payload change is a security fix)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] CLAUDE.md - `.claude/rules/collection-reads.md` names the id narrowing

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None recorded; design checked against the
collection-reads, ACL and capture-state rules before coding.
