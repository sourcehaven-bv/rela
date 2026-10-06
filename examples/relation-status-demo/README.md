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
- Moving a card replaces its `heeft_status` edge.
- A task's detail page and the board's side panel: Status is a field among
  the properties (`fields: - relation: heeft_status`), changed with a menu. `style_from: categorie` colours it by the
  status category.
