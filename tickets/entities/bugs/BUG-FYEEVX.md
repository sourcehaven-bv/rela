---
id: BUG-FYEEVX
type: bug
title: Frontend builds sub-resource URLs from the route id instead of the served face
description: Comments, duplicate, list script actions, export and staged uploads address the wrong row for a faced entity.
priority: medium
effort: s
why1: 'The SPA built sub-resource URLs from the route id or entity.id instead of the served address: comments appended _world.face (no longer sent), duplicate sent ?world= to the relations sub-resource (422), and documents, export, history, list script actions, staged uploads and link-from used the bare id.'
why2: servedRef is a local computed in EntityDetail and was applied site by site to writes (edit, delete, commands, detail actions) as defects surfaced; reads and other components kept their own addressing.
why3: The URL builders in src/api take a plain string entityId, so a bare id and a face address have the same type and nothing flags the wrong one.
why4: Faces were layered onto an id-addressed API and the faced e2e fixture (TKT-WCMW47) arrived late, so no test exercised a sub-resource on a non-default face.
why5: Addressing is a stringly-typed convention that neither the types nor the API-layer tests enforce.
prevention: EntityDetail.world.test pins comments, documents, export and history to the served address under a world with a bare route id; DuplicateModal, DynamicForm, useListActions and RelationPicker tests pin theirs; the relations helper no longer accepts a world; four faces-backlog e2e specs are enabled.
status: done
---

## Problem

Reported by the face-awareness inventory, not yet verified. Frontend code builds
addresses from the route id or `entity.id` instead of the served face address
(`servedRef`):

- the comments panel (`EntityDetail.vue:362-365`)
- the duplicate modal (`DuplicateModal.vue:148`)
- list script actions (`useListActions.ts:84`)
- the export button (`EntityDetail.vue:1878`)
- staged uploads

## Expected

Every sub-resource URL built for an entity uses the address of the face on
screen.
