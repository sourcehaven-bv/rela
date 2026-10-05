---
id: RR-0VLT7X
type: review-response
title: FamilyFaces is resolved outside the store transaction
finding: CreateRelation, UpdateRelation and DeleteRelationState read the family before the write, so a face created concurrently is not part of the FamilyFaces check for that one identity-edge write.
severity: minor
reason: The window is one write wide and the check matches ruling D4 for the family as read. Relation writes already authorize outside the Tx (BUG-K6FEVB ordering); moving family resolution into the Tx belongs with the relation API flip (TKT-KQXVF7 PR 7), not this PR.
status: wont-fix
---
