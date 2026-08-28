---
id: RR-F3QT0H
type: review-response
title: Redacted fields rendered writable in the section inline-edit path (isFieldWritable ignored the tombstone)
finding: |-
    computeFieldAffordancesFrom suppresses the writable verdict for hidden fields, so a tombstone is {visible:false} with writable == nil. The shared helper isFieldWritable (frontend/src/utils/affordances.ts) returned verdict?.writable !== false, which is TRUE for a tombstone.

    DynamicForm compensated independently (isFieldRedacted in isFieldReadonly), but the section inline-edit path did not: sectionEditFields.ts and SectionEditForm.vue contain zero references to 'visible'. sectionShouldRouteToInlineEdit routes to inline edit if any field isFieldWritable, and SectionEditForm sets writable: isFieldWritable(field.verdict).

    Scenario: entity has visible:false on priority. The entry properties section renders priority as an editable EMPTY widget (the value is stripped). User types; autosave PATCHes; the server returns 403 RuleFieldHidden. No data loss (the server fails closed) but a phantom editable control that always rejects. Regression introduced by this commit — before it, a hidden field had no _fields entry at all.
severity: significant
resolution: |-
    Fixed in the SHARED helper rather than per-consumer, since one consumer compensating while another did not is exactly what caused the gap: isFieldWritable now returns false when verdict?.visible === false.

    This makes every current and future consumer correct by default (DynamicForm, SectionEditForm, sectionEditFields, and anything added later). Pinned by three new tests in affordances.test.ts: redacted is not writable, stays non-writable even with an explicit writable:true, and an explicit visible:true is unaffected.
status: addressed
---
