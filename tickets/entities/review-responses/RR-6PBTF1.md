---
id: RR-6PBTF1
type: review-response
title: Title-cell href silently drops the query/scope context router.push builds
finding: 'The plan left ''extend entityDetailHref to build path + query'' as possibly optional. EntityList.navigateToEntity builds ~40 lines of reactive query state (from, scope, sort from live sortSpecs else default_sort, every filter[*] including arrays, and q). An href built separately in the template would be a bare /entity/<type>/<id>, so a plain click would carry the full scope and a cmd-click would not. The failure is silent: both paths render a valid page, but the new tab has dead prev/next and a wrong back target. Same applies to SearchView (from=search + q) and Kanban''s edit_form branch.'
severity: critical
resolution: 'Made mandatory rather than optional. Each surface now exposes one entityTarget()/cardTarget()/resultTarget() returning path AND query, consumed by both router.push and the link''s :to. Pinned by a test that derives the expectation from the actual push payload rather than a literal, so a new query param cannot make the two diverge silently. Mutation-verified: replacing :to with a bare /entity/type/id fails the two scope tests. Confirmed live in a browser — the popup URL matches the href character-for-character including from= and scope=.'
status: addressed
---

**Finding (C1, critical).** The plan leaves "extend `entityDetailHref` to build
path + query" as *possibly* optional. It must be mandatory.

`EntityList.navigateToEntity` (`:485-541`) builds ~40 lines of reactive query
state: `from`, `scope`, `sort` (live `sortSpecs` else `default_sort`), every
`filter[*]` key forwarded from `route.query` including array values, and `q`. If
the anchor href is built separately in the template it will be a bare
`/entity/<type>/<id>`.

Failure mode is silent: plain click carries full scope, cmd-click does not. The
new tab renders fine but `useScopeNavigation` prev/next is dead and the back
target is wrong. Nobody catches it in review because both paths navigate
successfully. Same applies to `SearchView.navigateToResult` (`from=search` +
`q`) and Kanban's `edit_form` branch.

**Resolution:** each surface exposes one `entityTarget(entity):
RouteLocationRaw`; `router.push(target)` and `router.resolve(target).href` both
consume it. Test the invariant, not the string:

```ts
expect(router.resolve(anchorHref).fullPath).toBe(router.resolve(pushedTarget).fullPath)
```

That one assertion subsumes the AC-7, `from=search`, `edit_form` and
`cellLink`-priority tests, and stays true when a query param is added later.
