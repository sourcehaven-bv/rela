---
id: TKT-WCMW47
type: ticket
title: E2E fixture and test matrix for faces and worlds
kind: chore
priority: high
effort: l
status: backlog
description: No e2e test declares faces or worlds; add a faced fixture project and cover attachments, export, documents, comments, history, relations, delete and copies.
---

## Description

No Playwright spec and no e2e fixture project declares faces or worlds. Frontend
unit tests mock the API, so they cannot see a server refusing a faced address.
Every server-side face bug in the inventory (BUG-CTUW2N,
`.ignored/face-awareness-inventory.md`) passed its frontend unit tests.

Add a faced e2e fixture project, for example derived from
`prototypes/worlds/project`, with at least two faces, a world, a file property,
a transform, an anchored document and a relation to a faced type. Cover, in
priority order:

1. Attachments: upload, download, delete, and sharing across a publish copy.
2. Entity and list export.
3. Anchored documents.
4. Comments on a face.
5. History and restore on a face.
6. Duplicate, and relation create to a faced target.
7. Delete of a single face and of the whole family.
8. Copies, publish and create-into-face.

Also make the realistic fixture (`facedApp` / `seedDeclaredFaceTicket`) the Go
default. Retire `seedDraftAndPublishedTicket`, which seeds a legacy bare row and
so lets zero-face reads pass.
