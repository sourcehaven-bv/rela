---
id: BUG-PFLS22
type: bug
title: Space Create menu on an entity page does not link the new item to the page entity
description: 'On an entity page the space Create menu creates the item without a relation to the page entity. A board or table tab''s New button does link the new row to the page entity over the tab''s relation. The Create menu does not, so an item created from a topic page does not appear under that topic. A timeline tab (scope: root) has no New button, so on that tab the Create menu is the only way to add an item and it never links. Expected: when the created type matches a scope of the page (a relation-scoped tab that shows that type, or a timeline hierarchy relation that starts at the page type), the new item is linked to the page entity over that relation, as New in a tab does.'
priority: medium
effort: s
why1: SpaceCreateMenu.onCreated only navigates to the new entity. It never creates a relation to the page entity.
why2: The menu does not know the page scope. It is mounted in the app header, and the page, tab and anchor exist only as PageView props passed down to the tab view.
why3: Linking a created row to the anchor (usePageTabScope.linkCreated) was built into the tab views (EntityList, KanbanView). It was not built as page state that other create surfaces can read.
why4: The sidebar payload says which relation each tab uses, but not which entity type the tab shows. A gantt root tab gives no relation at all. A surface outside the tab cannot map a created type to a relation.
why5: Create entry points were added one surface at a time (tab New, section Add, space Create menu). Nothing defines that every create made in the context of an entity page links to that entity, and no test covers create surfaces against that rule.
prevention: Create surfaces that run inside an entity page read one published page scope and one per-type link list from the server, so a new create button links like the existing ones. The Create menu test pins the rule for the menu.
started: "2026-10-05"
completed: "2026-10-05"
status: done
---

## Reproduction

Environment: data-entry SPA, any backend. Needs a space with a `create:` entry
and an entity page whose tab is scoped to that type.

```yaml
pages:
  topic:
    entity_type: topic
    label: Topic
    tabs:
      - { id: board, label: Board, kanban: task_board, scope: { relation: contains, direction: outgoing } }
      - { id: timeline, label: Timeline, gantt: portfolio, scope: root }
```

1. Open `/s/<space>/p/topic/<TOPIC-ID>/board`.
2. Choose Create > Task in the header, fill in a title and save.
3. The new task opens on its own page. It has no `contains` relation from the
topic, and the board tab does not show it.
4. Do the same on the `timeline` tab. Same result. That tab has no New button,
so there is no way to add a linked item from it.

New on the board tab, by contrast, links the new task to the topic.

## Root cause

`SpaceCreateMenu.vue` is mounted in the app header (`App.vue`), outside
`PageView`. Its `onCreated` only navigates to the new entity. The page scope
(page, tab, anchor) exists only as `PageView` props, which it hands to the tab
view. `usePageTabScope.linkCreated` is called by the tab views (`EntityList`,
`KanbanView`), so only those link.

## Fix plan

1. Server: make the link target for each created type explicit per entity
page. Add the row type to `SidebarPageTab` (from `pageTabRowType`) for
relation-scoped tabs. For a `scope: root` gantt tab, expose the hierarchy
relations that start at the page's entity type, with their child type.
2. SPA: expose the current page scope (page, tab, anchor id and face ref) from
a store that `PageView` sets and clears, so the header menu can read it.
3. `SpaceCreateMenu`: on an entity page, choose the binding for the created
type. Prefer the current tab's binding. Otherwise use the single binding on the
page for that type. If two relations on the page fit the type, do not link.
After create, link through the same code as `linkCreated`, including the "link
it by hand" message on failure. This also applies to each item created with "add
another".
4. Docs: `docs/data-entry.md` § Entity pages states that the Create menu links
as New does.

## Regression tests

- `SpaceCreateMenu` unit test: on an entity page, a created row of the tab's
type calls `createRelation` with the anchor, relation and direction. Off a page,
or for a type that matches no binding, no relation is created.
- Same test for a `scope: root` tab through a gantt hierarchy relation.
- Ambiguous case: two relations fit, so no link is made.
- Go test for the new `SidebarPageTab` fields.

## Related

`DynamicForm.getTypeFromId` splits an id at the first `-` and compares the
result with `id_prefix`. An `id_prefix` written with its trailing dash (`TKT-`,
the documented form) never matches. A section's "+ New" on a detail page then
aborts with "could not resolve the entity type of every related item" when the
form has no field for the pre-filled relation. This is a separate defect and
needs its own bug.
