---
id: RR-WRLDRL
type: review-response
title: The relations read is world-less while the create is world-scoped
finding: getAllEntityRelations takes no world parameter, while the modal threads world into the create and the rest of the app passes worldParam on reads. On a faced type the enumerated relations may not be those of the face on screen.
severity: minor
resolution: getAllEntityRelations takes an optional world and the modal passes props.world, so the enumerated relations belong to the face on screen.
status: addressed
---

## Suggested resolution

Pass the world through to the relations read.
