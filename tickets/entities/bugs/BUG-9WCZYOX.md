---
id: BUG-9WCZYOX
type: bug
title: The Changes list shows raw actors and relation names; History opens empty
description: Staff see raw actor ids and webhook names and forward relation labels from the target in the Changes list. History opens on an empty timeline and names the entity by type and id.
priority: medium
effort: m
why1: 'The Changes panel printed a record''s user as sent unless it was system: or integration:, so a person principal showed as PSN-019 and the IdP webhook as webhook:membership.created. An incoming relation without an inverse label showed the forward label with an arrow. The History button showed whenever the deployment had versioning, whatever the entity''s timeline held, and the history page printed type and id.'
why2: BUG-XR1DUQA built the titles map for relation ends only and named principals by prefix. The History button was gated on the capability and header_buttons only.
why3: The panel's tests used plain user names and inverse-labelled relations, and no fixture had a principal that is an entity id, a webhook identity or a relation without an inverse label.
why4: Projects whose principal_property resolves people to entity ids, and webhook identities outside the reserved prefixes, were not considered when the reader-facing panel was designed.
why5: Reader-facing labels were derived from the shapes in rela's own fixtures rather than from every principal and relation shape a project can produce.
prevention: Tests pin actor titles from the gated batch, labels for webhook and integration identities, incoming relations without an inverse label, and History hidden for an empty timeline in both header blocks.
started: "2026-11-20"
completed: "2026-11-20"
status: done
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
