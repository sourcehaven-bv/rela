---
id: RR-6PBTF1
type: review-response
title: Title-cell href silently drops the query/scope context router.push builds
severity: critical
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
