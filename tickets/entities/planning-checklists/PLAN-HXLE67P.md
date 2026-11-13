---
id: PLAN-HXLE67P
type: planning-checklist
title: 'Planning: Gated paged reads for MCP list_entities and search with no text'
started: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: MCP `list_entities` with no filter or a filter that lowers to property
equalities; search with no text (`Service.listAll`, the admitting path, and
`Visible.SearchVisible` with no text). Out: filters that do not lower
(TKT-Z099FGY), TKT-M58EQ1W, pgstore's native search (already paged in SQL).

**Acceptance Criteria:**
1. `list_entities` cost does not grow with the type: `TestHandleListEntities_ScaleFree`.
2. The total is post-gate: `TestScriptReader_ListEntityPage` (bob total 12, carol 0).
3. Search with no text stops at the limit: `TestSearch_NoTextIsScaleFree`, `TestSearchVisible_NoTextIsScaleFree`.
4. The allowlist entries for `handleListEntities`, `listAll` and `scanFaces` are gone.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach follows the existing pushdown read)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern exists)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `visibility.ScriptReader` pushdown (`composeReadScope`, `pushdownRedactor.redactPage`) gives the gated row scope as a `GraphQuery`.
- `store.CountMatched` and `GraphQueryHeaders` give a gated total and a header page.
- `queryplan.LowerScope` lowers a filter exactly to `Props`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
Add `visibility.ScriptReader.ListEntityPage(PageQuery) EntityPage`: the row
scope from `composeReadScope` plus the lowered Props go into one `GraphQuery`;
`CountMatched` gives the total and `GraphQueryHeaders` with `Limit`/`Offset`
the page, redacted once. Props are refused (`ErrPropsNotPushable`) when a
property is hidden on some rows. MCP uses it through `GraphReader`. Search
with no text reads keyset pages (`ListEntitiesPage`), merges types in id
order and stops at the limit. Alternative rejected: depend on #2247's
`scopeRequest.ByIDs` (not merged).

**Files to modify:**
`internal/visibility/entitypage.go`, `internal/appbuild/appbuild.go`,
`internal/mcp/server.go`, `internal/mcp/tools_entity.go`,
`internal/search/index.go`, `internal/search/visible.go`,
`internal/archguard/unpagedread_allowlist_test.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
MCP `type`, `filter`, `offset`, `limit`. The filter must lower exactly or it
runs in Go over gated rows; offset is clamped to 0; a page needs a type and a
positive limit.

**Security-Sensitive Operations:**
Row gate and field gate. The row scope is in the store query; the total is
counted under the same scope; Props on a field hidden on some rows are
refused, so a filter cannot probe hidden values (BUG-WJSYQT).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
See acceptance criteria; plus `TestHandleListEntities_PushedFilterMatchesGoFilter`.

**Edge Cases:**
Offset past the end; family split across a store page
(`TestSearch_NoTextAdmitsWholeFamilies`); no type (every schema type).

**Negative Tests:**
Unbounded page query refused; hidden-field filter refused; reader with no
grant gets an empty page and total 0.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
Order changes from natural sort to byte order (noted in the PR). Golden read
counts change (regenerated, counter lines only). Effort: m.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: tool contract unchanged)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: reviewed by the code reviewers on the diff instead)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see review responses)

**Design Review Findings:** N/A
