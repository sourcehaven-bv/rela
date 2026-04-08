---
id: RR-7NAS
type: review-response
title: Collision with existing filter_* (underscore) URL params
finding: 'EntityList.navigateToEntity and useScopeNavigation already use filter_<prop>=value format. Adding filter[<prop>] creates two conventions on the same surface. Concrete bug: list URL uses brackets, click entity → navigateToEntity writes underscore form, click back → bracket form. Scope nav reads underscore format and gets a different result set than the list view shows.'
severity: critical
resolution: Migrate useScopeNavigation, navigateToEntity, and SearchView to read/write the bracket format. Single useUrlFilterSync composable owns the round trip. No backwards compat for the underscore form.
status: addressed
---
