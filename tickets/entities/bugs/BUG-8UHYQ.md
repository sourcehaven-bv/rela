---
id: BUG-8UHYQ
type: bug
title: Detail-view list section items are not clickable (no href, broken router push)
description: |-
    In the data-entry SPA, sections with display: list render items as <a class="list-link"> with no href and a click handler that calls router.push({ name: 'entity', params: { id } }) — but the 'entity' route requires both :type and :id. The push fails (silently swallowed by router.onError), so clicking does nothing. The same broken navigateToEntity is used for the cards / content-cards displays, but the list display is the most visible because the <a> looks like a link.

    The expected behavior is to navigate to the linked entity's detail-view (mirroring EntityList: column link > list.detail_view > /entity/:type/:id).

    GitHub issue: https://github.com/sourcehaven-bv/rela/issues/647
priority: medium
effort: s
why1: navigateToEntity called router.push({name:'entity', params:{id}}) but the route requires :type/:id; vue-router rejected with MISSING_REQUIRED_PARAMS and router.onError silently swallowed it.
why2: The function accepted only entityId even though the API response carries entity.type per item -- the caller had the type but the function ignored it.
why3: No automated test exercised the click path on a custom view's list/cards/content section, so the broken navigation drifted in unnoticed when the route shape grew :type.
why4: The 'navigate to entity X' logic was duplicated across EntityList, SidePanel, RelationCards, and CustomView -- three of those used path strings (correct after route grew :type), one used a named-route push (broke). The duplication let one consumer drift while the others stayed correct.
why5: There was no first-class config concept for 'where do you view an X' -- detail_view lived on per-list config, so consumers without a list anchor (CustomView sections) had no clean lookup. Adding entity_views.<type>.detail_view + a shared entityDetailHref helper removes the drift surface entirely.
prevention: New entity_views.<type>.detail_view config concept centralizes 'where to view an X' across CustomView and EntityList via a single entityDetailHref helper. Vitest component test for CustomView locks in the rendered href + click contract. CI guard test walks the repo for data-entry.yaml files and asserts migration Detect()=false.
status: done
---
