---
id: RR-4IB3OQ
type: review-response
title: Full-view refetch, no budget test, and repeated form-list sorting on the new per-section path
finding: Three cost issues. (1) AC3 said the modal flow "refreshes the section in place", but EntityDetail.vue:666-698 loadView() refetches the whole view and replaces viewData wholesale — there is no per-section fetch, so "in place" only means no route change. (2) CLAUDE.md requires a storetest.Counting budget test for new read paths and querybudget_test.go:207 already has TestQueryBudget_ViewTableSectionIsSizeIndependent covering this exact path; the plan had none. (3) createFormForType (views_handler.go:772-790) builds and natsorts the full form-id list on every call — per section, per target type — and the new AuthorizeWrite runs per candidate type per section, repeating for types reachable by several relations. A type with 20 relations means 20+ full form-list sorts per page load.
severity: significant
resolution: 'AC3 reworded to say the refetch is whole-view and that "in place" means no navigation. AC14 added requiring the _views count be flat at 10 and 50 rows with create: enabled, extending the existing querybudget fixture. Plan now requires memoizing both createFormForType and the create-permission check per request, keyed by entity type, rather than discovering the cost later.'
status: addressed
---

## Finding

**Full-view refetch.** AC3 said the modal flow "refreshes the section in place".
`EntityDetail.vue:666-698` `loadView()` refetches the whole view and replaces
`viewData` wholesale — there is no per-section fetch. "In place" means only "no
route change", which is a weaker claim than the wording implied.

**No budget test.** CLAUDE.md requires one for new read paths, and
`querybudget_test.go:207` `TestQueryBudget_ViewTableSectionIsSizeIndependent`
already covers this exact path. Button resolution adds per-section work to it.
None of that work is row-dependent, so the existing tests should stay green —
which is precisely why asserting it is nearly free and worth doing.

**Repeated work per section.** `createFormForType` (`views_handler.go:772-790`)
builds and `natsort`s the **full form-id list on every call**, per section per
target type. With the synthesized view emitting one section per relation, a type
with 20 relations means 20+ full form-list sorts per page load. The new
`AuthorizeWrite` similarly runs per candidate type per section and repeats for
types reachable by several relations.

## Resolution

- AC3 reworded: the refetch is whole-view; "in place" means no navigation. It also
now carries the create-without-read exception (toast naming the id, since a full
refetch has no cache to seed from, unlike TKT-OMUD56's widgets).
- AC14 added: `_views` count flat at 10 and 50 rows with `create:` enabled, extending
the existing fixture.
- Plan now requires memoizing both `createFormForType` and the create-permission check
per request, keyed by entity type — named up front rather than discovered in
review.
