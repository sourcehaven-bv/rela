---
id: RR-E7Y7TO8
type: review-response
title: Proposal dialog can apply to an entity the user left
finding: The llm action reply was shown whatever entity was on screen when it arrived; the dialog used the current entry type with the old proposal id.
severity: significant
resolution: EntityDetail captures the address and type at run time, drops a reply for an entity the user left and closes the dialog on navigation. Tests in EntityDetail.actions.test.ts; the drop test fails with the guard removed.
status: addressed
---
