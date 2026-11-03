---
id: BUG-D8AM2A
type: bug
title: Entity ETag hashes hidden field values
description: The entity ETag hashed hidden field values. Fixed on develop by BUG-J01V0Q.
priority: medium
status: wont-fix
---

> Closed: fixed on develop by BUG-J01V0Q (#1867). After the TKT-0M4H0S rebase, the corpus ETag hashes only visible fields.

## Description

The entity ETag hashes hidden field values. `entityETagWithEdges(entity,
outgoing)` (internal/dataentry/api_v1.go:1047, function at :2494) hashes the
row-gated but unredacted entity, so a viewer whose `note`/`secret` are redacted
gets the same ETag as an admin (recorded in the TKT-0M4H0S corpus,
`item-closed`).

The viewer knows every other input to the hash, so for a hidden enum, date,
number or boolean it can test candidates offline or with `If-None-Match` (304).
For any field, a changed ETag with unchanged visible data reveals that a hidden
value changed. CalDAV already avoids this (caldav_backend.go:551).

## Fix

Hash the redacted entity actually served; make the write-path If-Match check
(write_handler.go:1054) compute the same per-principal validator. Pin with a
test that two principals with different field visibility get different ETags and
that a hidden-only change leaves the viewer's ETag unchanged.
