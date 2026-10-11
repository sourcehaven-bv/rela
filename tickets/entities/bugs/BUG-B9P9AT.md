---
id: BUG-B9P9AT
type: bug
title: Property rename or drop in a data migration leaves property-anchored comments pointing at a missing property
description: rename_property and drop_property rewrite entity rows below the entitymanager but leave comments anchored to the old property name, so the anchor points at a property the type no longer declares.
priority: low
status: backlog
---

## Reproduction

A comment anchored to a property (`anchor: {kind: property, ref: owner}`) is
stored on an entity. A data migration runs `rename_property` (owner → assignee)
or `drop_property` (owner).

The comment keeps `ref: owner`, a property the type no longer declares. The
thread is still at the right address (BUG-6OZBP9 covers addresses); only the
anchor is stale.

## Expected

`rename_property` re-points property anchors to the new name. `drop_property`
leaves them detached in a way the UI shows as detached, the same as a text
anchor whose quote is gone.

Found in the code review of BUG-6OZBP9 (RR-5AUXXX).
