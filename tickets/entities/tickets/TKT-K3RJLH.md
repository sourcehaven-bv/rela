---
id: TKT-K3RJLH
type: ticket
title: 'Piles: personal working sets of entities (service, API, scope source, UI)'
kind: enhancement
priority: medium
effort: xl
started: "2026-10-08"
completed: "2026-10-09"
status: done
description: 'Add per-user piles: collect entities from lists and search into a named set, step through it, run configured actions on it and export it.'
---

## Description

A pile is a named, personal set of entities. A user fills it from any list or
search selection, keeps it in the sidebar, and works through it: steps through
it on the entity page, runs operator-configured actions on it, and exports it.
Feature FEAT-4FLTQ9; Atlas TASK-KXEES. The mockup is
`frontend/packages/rela-components/src/mockups/PileMockup.vue` (Storybook
"Mockups/Pile").

### Decisions so far

- **Single-player.** A pile belongs to one principal. Only the owner reads or
changes it. A collection a team should see is graph data (an entity with
relations); a later "Turn into entity" action can bridge the two.
- **Outside the graph.** A pile is a fact about one person, like
`internal/userstate`. Adding to it is not an entity write: no audit record, no
version capture, no automation.
- **Its own service, not a `state.KV` blob.** Membership changes are set
operations (add, remove), so two tabs cannot overwrite each other. TKT-DK0X6O
made the same move for scheduler run state.
- **Generic.** No "reviewed" or other workflow state on items.
- **Ids only.** An item is an entity id; the request's world picks the face.
Reads go through one batched visibility resolution. A hidden item is left out,
not deleted. Counts and the 500-item cap use the readable set.
- **Never in `rela.db`.** Postgres gets its own tables; fs and sqlite (desktop
included) keep piles in a node-local `.rela` file, so a shipped database does
not carry them.
- **A pile is a scope source** beside `list` and `search`
(`internal/dataentry/scope.go`), so prev/next navigation and export reuse the
existing ACL-gated paths.
- **Batch (aggregate) actions are deferred.** Pile actions run per item, like
list actions.

### In scope

- `internal/piles` service with a node-local KV backend and a postgres
backend, plus a conformance suite.
- REST API for piles and their items.
- `pile` scope source: step through a pile on the entity page.
- SPA: add to pile from list and search selections, new-pile dialog, sidebar
entries with counts, the pile panel with tick-to-remove and undo.
- `piles:` block in `data-entry.yaml` naming the actions and transforms a pile
offers; running them per item, exporting through the list-export path.

### Out of scope

- Sharing piles, and "Turn into entity".
- Batch actions that receive all items in one script call.
- MCP tools and CLI commands for piles.
