---
id: RR-ZHSCP1
type: review-response
title: Calendar preview opens the edit form on a bare id
finding: CalendarView opens EntityPreviewModal with ev.entity.id, so the edit form gets a bare id for a faced entity.
severity: significant
resolution: CalendarView opens the preview with entityRef(ev.entity), so the preview and its Edit use the address the calendar showed. Unit test added in CalendarView.test.ts.
reason: 'Pre-existing face-address defect unrelated to the world: the change here only adds the world. Recorded as a follow-up on the bug.'
status: addressed
---
