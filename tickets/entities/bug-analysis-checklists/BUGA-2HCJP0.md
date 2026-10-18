---
id: BUGA-2HCJP0
type: bug-analysis-checklist
title: 'Analysis: Free-text search judges hidden fields one hit at a time'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally. On origin/develop (bc814667d),
`TestQueryBudget_ListFreeTextUnderFieldHiding` passes only because it pins the
per-hit cost: at 10 and 50 hits it asserts exactly one field-check entity load
and one `MatchingFaces` traversal per hit.
- [x] Minimal reproduction steps documented. Policy: role `editor` with
`read: ["*"]` and `visible: {ticket: [title, {field: status, when:
related(...)}]}`. Seed n tickets and list `GET /api/v1/tickets?q=Ticket` as the
editor. The store counter shows n extra `GetEntity` calls and n `MatchingFaces`
calls.
- [x] Environment/conditions noted. Any free-text path under a `visible:`
policy: list `?q=`, `/_search` and the MCP searcher. The cost is per hit in
`search.Visible` (memstore, fsstore, sqlite); pgstore already batched bodies,
but every backend paid one `HiddenFieldsFunc` traversal per hit.

## Root Cause

- [x] Immediate cause identified (why1): `search.Visible.fieldVisible` judged
one hit at a time. For each hit it loaded the entity and called the consumer's
`HiddenFieldsFunc`, which evaluated the grant's `when:` for that one row.
- [x] Contributing factors found (why2-3): `HiddenFieldsFunc` was a per-row
callback from the store into the ACL, with no batch step. It was the only way
the search layer could ask which fields a row hides, because field visibility
was decided in Go per row rather than compiled into the query.
- [x] Systemic cause explored (why4-5): a `visible:` `when:` could be any
predicate, including ones the store cannot answer (`not`, `has_role`), so a
per-row callback was the only general evaluation. Nothing required the condition
to lower to a store query.

## Fix Planning

- [x] Fix approach determined. Compile field visibility into the query
(architecture branch commit "compile field visibility into the query"):
`affordances.PolicyResolver.SearchFieldScope` builds a `search.FieldScope` per
type, with the gated fields and one `store.GraphQuery` per conditional grant.
`search.Visible` reads a page's rows in one `ListEntities` and answers each
grant with one `MatchingFaces` per type per page. `HiddenFieldsFunc` and
`FieldVisibleSearcher` are deleted. Every `visible:` `when:` must lower through
`queryplan.LowerScope` or the policy fails to load.
- [x] Regression test planned. `TestQueryBudget_ListFreeTextUnderFieldHiding`
becomes a size-independence assertion: zero field-check entity loads and the
same traversal count at 10 and 50 hits. `storetest.RunFieldScopeSearchTests`
pins the behaviour on every backend.
- [x] Related areas checked for similar issues. `/_search` and the MCP
searcher run the same `visibility.SearchScoped` path. pgstore computes the grant
predicates inside the search SQL. sqlite and fsstore use `search.Visible`.
