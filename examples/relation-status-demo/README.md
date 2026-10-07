# Relation-backed status demo

Demo for TKT-KJ3Q07 / RES-8CKUNJ: kanban columns from a single-valued
relation (`columns_from`).

```
cd examples/relation-status-demo
rela --project . script scripts/seed.lua   # once
rela-server -project . -port 8957
```

- `/p/initiatief/<INIT-id>/bord`: the columns are the statuses the initiative
  offers (`biedt_status`), in that relation's order. Klantportaal and
  ISO-audit offer different sets. A task whose status the initiative does not
  offer appears under "Other".
- `/kanban/alle_taken`: every status, ordered by `volgorde`.
- `/list/taken` and the initiative's `lijst` tab: tasks grouped by status
  (`group_by: {relation: heeft_status}`). Add in a section prefills the status.
- Each column heading takes its colour from the status category
  (`style_from: categorie` on `columns_from`, through the `statuscategorie`
  styles). Add in a column creates a task with that status; on the
  initiative's board the task is also linked to the initiative.
- Moving a card replaces its `heeft_status` edge.
- A task's detail page and the board's side panel: Status is a field among
  the properties (`fields: - relation: heeft_status`), changed with a menu. `style_from: categorie` colours it by the
  status category.
