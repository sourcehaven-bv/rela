---
id: BUG-YMC587J
type: bug
title: The Changes list shows raw actors and relation names; History opens empty
description: Staff see raw actor ids and webhook names and forward relation labels from the target in the Changes list. History opens on an empty timeline and names the entity by type and id.
priority: medium
effort: m
status: ready
---

## Description

On a staff page the Changes list and the History button still show raw names:

- a person who made a change shows as their entity id (`PSN-019`);
- the identity provider webhook shows as `webhook:membership.created`, and
other integration identities collapse to "System" with no name;
- a relation seen from its target shows the forward label with an arrow
(`Completes signup of ← owner@…`), even where the schema defines an inverse
label, and reads backwards where it does not.

The History button opens "No versions recorded yet" when the entity has no
versions, and the history page names the entity as `account · ACC-019` instead
of by its title.

## Acceptance Criteria

- [ ] An actor whose principal is an entity id the reader may read shows
that entity's title; one the reader may not read shows as sent.
- [ ] `webhook:<event>` shows as the identity provider with the event,
`system:webhook:<id>` as a webhook with its id, and `integration:<name>` as an
integration with its name. Other `system:` principals stay "System".
- [ ] A relation seen from its target uses the inverse label when defined,
and otherwise names the other entity as its source of the forward label.
- [ ] The History button is hidden when the entity has no versions.
- [ ] The history page names the entity by its title.
- [ ] The behaviour is documented.
