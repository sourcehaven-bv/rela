---
id: PLAN-UTJ81VF
type: planning-checklist
title: 'Planning: Store-side paging for list_entities filters that do not lower'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: MCP `list_entities` with a filter. The filter lowers through
`queryplan.LowerCondition` (via appbuild's `lowerQueryCondition`, the same
lowering the data-entry list and query scopes use): property equalities, `~=`,
`related(...)`, `not related(...)`, `current_user` comparisons and
`has_current_user`. Traversals are gated per request with the traversal gate the
Go path's binder already uses. The query identity is stamped per call so
`current_user` lowers (and evaluates in Go). Out: ordered comparisons, `or`,
`not` over a non-traversal and equalities on a property hidden on some rows.
They keep the Go path, which still reads the whole readable type, because the
response carries an exact total and that needs every row judged. The allowlist
entry stays with an honest reason.

**Acceptance Criteria:**
1. A lowered filter (`related`, `not related`, `~=`, `current_user`) costs the same over 100 and 1,000 rows: `TestHandleListEntities_ConditionScaleFree` (AssertScaleFree), which fails with the lowerer removed.
2. The store path and the Go path give the same rows and totals for every principal and filter, on memstore, sqlite and postgres: `TestListEntities_ConditionMatchesGoPath{,_SQLite,_Postgres}`.
3. A positive traversal the gate denies is an empty page with no row read.
4. A filter over a field hidden on some rows stays on the Go path for that principal.
5. `current_user` filters answer for the caller (they failed with "no current user" before).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: reuses the TKT-I39Y7N4 lowering)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern exists)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `appbuild.lowerQueryCondition` (conditionlower.go): LowerCondition plus per-request traversal gating, with the denied/refused outcomes.
- `dataentry.lowerViewCondition` (listpushdown.go): pushes only when every attribute is visible on every row; MCP gets the same check from `visibility.ScriptReader.ListEntityPage` (`ErrPropsNotPushable`).
- `appbuild.nextActionRequestScope`: the one query-identity derivation, used by query scopes.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `visibility.PageQuery` gains `Related []store.DirectedRelation` (already gated by the caller), ANDed onto the read scope in `readPage`; `pageViaFilter` refuses it.
- appbuild: `GatedReadBundle.Conditions` (`QueryConditions{meta, gate}`) with `StampQueryIdentity` (nextActionRequestScope) and `Lower` (lowerQueryCondition with the bundle's traversal gate, the same gate as `Traversals`).
- mcp: consumer-side `ConditionLowerer` on `RelatedReads.Conditions` (nil accepted: every filter in Go). `handleListEntities` stamps the identity when a filter is present; `listPage.ofType` lowers, returns an empty page for a denied positive traversal, pushes Props and Related, and falls back to `filterInGo` on `ok=false` or `ErrPropsNotPushable`.
- Wiring: `internal/cli/mcp_wiring.go`, `internal/dataentrywire/mcp.go`.
- Alternative rejected: chunked header pages for the Go path. The total still needs every row, so rows read would still grow with the type, and offset paging adds an O(n^2) cost on postgres.
- Alternative rejected: moving lowerQueryCondition into relresolve. A consumer-side interface keeps MCP off queryplan/identity details and reuses the function as-is.

**Files to modify:** internal/visibility/entitypage.go,
internal/appbuild/{appbuild.go,conditionlower.go},
internal/mcp/{server.go,tools_entity.go}, internal/cli/mcp_wiring.go,
internal/dataentrywire/mcp.go, internal/archguard/unpagedread_allowlist_test.go,
tests in internal/mcp.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The filter expression comes from the MCP client.
It is compiled by `compileListFilter` (unchanged); lowering is an exact
allowlist of shapes, anything else stays on the Go path.

**Security-Sensitive Operations:** Each traversal passes the request's traversal
gate before it becomes a `GraphQuery.Related` entry; a denied positive one
yields an empty page, a denied negated one drops out, a refused one falls back
to the Go path, which reports the same error. Props are pushed only when visible
on every row (`ErrPropsNotPushable`), so a hidden value cannot decide
membership. The total is the gated `CountMatched`. The identity is derived from
the stamped principal; no identity means `current_user` does not lower and fails
closed in Go.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1: AssertScaleFree over the handler for each lowered
shape. AC2-4: differential over principals x filters x pages, asserting which
path served (CountMatched present or not). AC5: a `current_user` filter returns
the caller's rows.

**Edge Cases:** Offset past the total; `related(x) and not related(x)` (no
rows); principal with no read on the type; no principal (current_user fails
closed).

**Negative Tests:** Filter with `or`/ordered comparison stays on the Go path and
still answers; principal without status visibility keeps the Go path.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Store and Go paths could disagree (wrong rows or totals): mitigated
by the differential on three backends. Gate world: both paths use the same gate,
so they agree. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs-project GUIDE-mcp-server: note that `current_user` works in a list_entities filter, if the guide mentions filter limits.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design reuses the reviewed TKT-I39Y7N4 lowering; the code review covers it)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A
