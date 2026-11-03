---
id: RR-UX4UO8
type: review-response
title: next_cursor named hidden entities and faces
finding: MentionPage.Next was built from raw index refs. On a full page it carried the last mentioning face of an id, which could be a face the caller may not read; when the scan budget ran out it was the last raw row, usually a hidden entity. Both reached clients as next_cursor.
severity: significant
resolution: 'Next is now a string: the id of the last served item, never a raw row and never a face. ParseMentionCursor accepts only a bare id. Tests: TestResolver_MentionedByCursorNamesNoHiddenFace and TestResolver_MentionedByBudget (all rows unreadable gives no cursor).'
status: addressed
---
