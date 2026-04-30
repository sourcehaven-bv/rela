---
id: TKT-603FQ
type: ticket
title: Add search interface to data-entry list views
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem\n\nData-entry list views only expose the filter widgets declared in `data-entry.yaml` under `filter_controls`. Users cannot type a free-text query to narrow the visible rows, and they cannot add ad-hoc filters on properties that the list config did not pre-declare. Free-text search exists only on the standalone `/search` route, which leaves the current list context (sort, page, etc.) behind.\n\n## Goal\n\nMake every list view searchable in place: a text search box that filters rows by id/title/property text, with the existing `FilterBar` continuing to drive structured filters. Optionally allow users to add filters on additional properties beyond those pre-configured.\n\n## Acceptance criteria\n\n- A search input is visible above every list view (`/list/:id`).\n- Typing in the search box filters the currently displayed list, debounced to avoid a fetch per keystroke.\n- The search query is reflected in the URL (deep-linkable + back/forward safe) and in `useScopeNavigation` context when entering a row.\n- Clearing the search restores the full filtered list.\n- The existing `filter_controls` widgets continue to work and are AND-ed with the search.\n- Empty result is rendered with a helpful message and a clear-search affordance.\n- Keyboard: `/` focuses the search box; `Esc` clears and blurs.\n- Backend filters do not regress: pre-configured `configuredFilters` from list config still apply.\n\n## Out of scope\n\n- Saved searches / named queries.\n- Adding new operators beyond what `filterStateToApiParams` already supports.\n- Replacing the standalone `/search` route.\n
