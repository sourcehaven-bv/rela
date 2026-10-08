---
id: BUG-EEIIXB
type: bug
title: Relation-conferred type@face grant denies explicitly addressed faces at the row gate
description: A role conferred by a relation and granted only type@face fails PermitsRead for the entity, so getVisibleRef 404s an explicit ID@face address to the granted face on the entity and comment routes.
priority: medium
why1: 'Before #1753 PermitsRead ran a conferred role''s ReadQuery through store.MatchingIDs in the default world. The query''s face allowlist (ticket@draft) excluded the default-face row, so PermitsRead answered false and getVisibleRef returned 404 before it loaded the addressed face.'
why2: 'The row gate and the face gate answered their questions against different rows: the row gate assumed that ''may read some face of this id'' could be asked of the default world alone.'
why3: Per-branch face allowlists for conferred roles (TestReadQuery_ConferredRolesKeepTheirOwnFaces) came after PermitsRead was written as face-blind, and nothing re-checked that assumption.
why4: 'Tests of conferred grants used whole-type grants (read: [ticket]), so the face-limited conferred case never ran end to end.'
why5: No test crossed grant source (global vs conferred) with grant breadth (whole type vs type@face) on the explicit-address routes.
prevention: 'Fixed upstream by #1753 (TKT-7IZHP0): PermitsRead is now ReadableFacesMany asked over every face. TestComments_FaceLimitedConferredGrant pins the entity and comment routes for a conferred ticket@draft grant; it fails on the commit before #1753 with the reported 404s.'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

## Reproduction

Policy: role `viewer` with `read: [ticket@draft]`, conferred by the relation
`owned-by`. Alice owns TKT-1, which has a default face and a `draft` face.

- `PermitsRead(ctx, "ticket", "TKT-1")` returns false.
- So `GET /api/v1/entities/ticket/TKT-1@draft` and
`GET /api/v1/_comments/ticket/TKT-1@draft` both return 404, although the grant
names exactly that face.
- With `read: [ticket]` (every face), both return 200.

Found while writing the regression tests for BUG-R1PQY9.

## Suspected cause

`PermitsRead` is documented as face-blind, but for a relation-conferred role it
runs the composed `ReadQuery` through `store.MatchingIDs` in the default world.
Since the per-relation branches
(`TestReadQuery_ConferredRolesKeepTheirOwnFaces`), each branch carries its
role's faces, so the query matches the default-world row only when the role
grants the default face. `getVisibleRef` calls `PermitsRead` before it loads the
addressed face, so an explicit address to a granted non-default face is denied
at the row gate.

A global assignment is not affected: `ReadQuery` returns AllowAll and only the
face gate applies.
