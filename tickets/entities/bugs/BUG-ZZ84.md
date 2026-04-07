---
id: BUG-ZZ84
type: bug
title: Entity delete navigates to broken /list/{type}s URL
description: When deleting the last entity of a given type in the data-entry UI, the frontend redirected to /list/${entityType}s which assumes the list ID is the entity type plural. For any project where lists have different names (e.g. PIM uses all_daily_notes for daily-note entities), this shows 'List not found' instead of the expected empty list view.
priority: medium
why1: After deleting an entity, EntityDetail.deleteEntity pushed /list/${entityType}s which assumes the list ID matches the entity type plural.
why2: PIM (and any project where lists are named differently) has lists like all_daily_notes for daily-note entities, so the URL /list/daily-notes doesn't exist and the frontend shows 'List not found'.
why3: The delete navigation was hardcoded instead of reusing scope navigation info or the schema's list-to-entity-type mapping that was already available via the schema store.
why4: Frontend components were allowed to hardcode list URLs based on entity type assumptions instead of consulting the schema store, which had no API for that lookup.
why5: The schema store API grew organically without a clear principle that all cross-references should go through typed getters that would catch missing configurations at lookup time.
prevention: Added findListIdForEntityType getter to the schema store so any component can look up a list for an entity type. EntityDetail now uses scopeNav.backUrl if available, falls back to the configured list for this entity type, and finally the dashboard. Same fallback applied to the 'entity not found' back link.
status: done
---

When deleting the last entity of a given type in the data-entry UI, the frontend
redirected to `/list/${entityType}s` which assumes the list ID is the entity
type plural. For any project where lists have different names (e.g. PIM uses
`all_daily_notes` for `daily-note` entities), this shows "List not found"
instead of the expected empty list view.
