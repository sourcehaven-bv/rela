---
id: RR-FFXKEK
type: review-response
title: Configured lists that sort, group or filter on a conditionally hidden field fail
finding: 'Admins are refused sort/filter on fields a conferrable role hides. A configured list whose sort:/group_by:/filters: names such a field now fails for those principals, and the SPA has no property_not_visible handling.'
severity: significant
resolution: 'Root cause fixed in BUG-Y5UF3K: FieldVerdicts unioned roles non-monotonically, so a conferrable local role could hide a field an admin role without a block showed. FieldVisibleOnEveryRow was rewritten on the monotonic rule: with a global role reading the type, each face a global role reads needs a block-less global reader or an unconditional global grant; local roles are consulted only when no global role reads the type. ConferrableRoles was removed. Admin, editor and the JWT stranger get 200 again on list-filter-hidden*/list-sort-hidden*, byte-identical to the pre-fix recordings; the viewer is still refused. Kept the SPA unsorted fallback, since a configured list sorted on a field a viewer cannot see still gets 400 for that viewer. Kept the load-time list lint, reworded: it still flags real cases (a list sorting or filtering on a field a closed visible: block such as the watcher role hides).'
status: addressed
---

## Interim changes (left open for the semantics decision)

- SPA before this change: any list error replaced the whole list with the
problem detail and a Retry button (EntityList), or an error status region
(FlyoutList). There was no handling specific to `property_not_visible`.
- SPA now: `withSortFallback` in `frontend/src/api/entities.ts` retries a read
refused with `property_not_visible` once without `sort`. EntityList shows an
info banner ("Shown unsorted: ..."). A refused filter still shows the error,
because dropping a filter would widen the list.
- Load-time lint: `warnListVisibilityConflicts` (listfieldgate.go) warns at
startup and on config reload for each list whose `sort:`, `group_by:` or
`filters:` names a property some role's `visible:` block does not grant
unconditionally.
