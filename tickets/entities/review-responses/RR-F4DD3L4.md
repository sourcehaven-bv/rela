---
id: RR-F4DD3L4
type: review-response
title: Editing or removing an open question is not covered
finding: Only the session-moving message is locked against its author; other questions can be edited or deleted after answers, and a removed open question can never be answered, so the session waits forever.
severity: significant
resolution: 'Plan: author edits and deletes of a question in asked are refused while the span lasts, as for the session message. A moderator removal drops the question from asked; the session then moves on the next post, not on the removal (documented). Tests added.'
status: addressed
---
