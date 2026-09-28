---
id: BUG-FYEEVX
type: bug
title: Frontend builds sub-resource URLs from the route id instead of the served face
description: Comments, duplicate, list script actions, export and staged uploads address the wrong row for a faced entity.
priority: medium
effort: s
status: backlog
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
