---
id: BUG-494HRY
type: bug
title: Edit form drops the world and hides relations to faced peers
description: An edit form opened from a world loads the entity in the default world, so relations to a faced peer that only that world serves are missing from the form.
priority: high
effort: s
why1: The edit form fetched the entity without a world, so the server resolved its neighbours in the default world and dropped the peers that have no face there. The picker showed no tile for them.
why2: Every edit entry point built the form route from the address alone and dropped the page's ?world=, and DynamicForm.loadEntity never passed a world to fetchEntity.
why3: The form treated the world as irrelevant because the edited row is named by its address, which is literal in every world. That is true for the row, but the entity GET resolves the row's relations per world, so the relation set the form loads depends on the world.
why4: Detail and create routes carry the world (create since BUG-HC6I2T), but each call site spells its own route, and there was no shared helper or test for the edit route. Faced specs opened edit forms by direct URL, never from a world-bound page.
why5: 'World propagation is a per-call-site convention rather than one function, so each new entry point can drop it silently. Until #1753 the picker also saved a full-replace set, which turned this display gap into data loss.'
prevention: Edit routes are built by one helper, editFormRoute, which takes the world as an argument, so a new entry point cannot build an edit link without deciding on the world. The form reloads when the world changes. An e2e spec opens the form from a world-bound page and checks a relation only that world serves.
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

## Problem

An edit form does not show relations to a faced peer that the world on screen
serves but the default world does not.

Example: a faceless `task` links to two `procedure` entities through `about`.
`procedure` has faces `draft` and `approved`; the procedures exist only as
drafts. The detail page in a world that selects `draft` lists both links. Click
Edit: the "About" picker is empty. Add a third procedure and save.

## Expected

The form shows both existing links, and after the save the task has three.

## Observed

- The form shows no links, so the user cannot see or remove them.
- On releases up to v26.10.1 the save also deleted the two hidden links: the
picker sent the full relation set (`{data: [...]}`), which replaces. Since
  #1753 (v26.10.2) the picker sends a delta (`add`/`remove`) against the
loaded set, so a hidden link is no longer deleted. The display defect remains.

## Cause

Every edit entry point (detail Edit button and shortcut, section row edit, list,
kanban, side panel, calendar preview, document view) routes to
`/form/:id/:entityId` without `?world=`, and `DynamicForm.loadEntity` fetches
the entity without a world. The entity GET resolves neighbours in the world it
is asked for and drops a neighbour with no face there, so the form loads in the
default world, not the world the user was looking at. The picker's candidate
search reads the world from the route, so it was in the default world too.

## Fix

Every edit entry point builds its route with `editFormRoute`, which carries the
page's `?world=`. `DynamicForm` loads the entity in that world. The form then
shows the relations the page showed, and the picker searches the same world.

### Interaction with per-field versions (#1759)

The relations version token hashes the relations as the read's world serves
them, and a PATCH checks it against the default world's view. A form read in
another world therefore got a 412 on every relations save. The conflict path
then refetched with `?world=default`, which a schema that declares worlds
refuses with a 400. The form now drops the relations token when it read in a
non-default world (the relations body is a delta, and the save's response
brings a token in the write's view), and the refetch omits `?world=` so the
server reads in its default world.

## Follow-up

The cards relation widget reads `/relations/{rel}`, which serves the default
world only. It has the same display gap for a faced peer that only another world
serves. Its saves are deltas, so no edge is lost.

The side panel, the calendar preview and the document view open the edit form on
a bare id, which a faced type refuses to write (RR-UP6T3R, RR-ZHSCP1,
RR-YGZSSR). The side panel and document view also read in the default world.
