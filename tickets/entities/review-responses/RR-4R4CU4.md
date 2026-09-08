---
id: RR-4R4CU4
type: review-response
title: EntityList delete confirm said "Delete Entity" while the mutation deletes one face
finding: The list's delete mutation was changed to address the row (`entityRef`), so on a non-bare row it removes one face, but the confirm dialog still warned about deleting the whole entity — wrong in both directions.
severity: significant
resolution: 'EntityList.requestDelete now branches on the row''s face like EntityDetail: ''Delete Face?'' naming the face label and stating that the other faces are kept.'
status: addressed
---
