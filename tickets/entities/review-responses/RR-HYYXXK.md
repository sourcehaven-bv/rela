---
id: RR-HYYXXK
type: review-response
title: Double-submit window stays open unless the reset is awaited inside the try; Cmd+Enter bypasses PendingButton
finding: 'The reset must be async (it awaits loadTemplates and refreshStagedAffordances). If it is not awaited INSIDE handleSubmit''s try, there is a window where saving===false and createdEntityId===null while the reset is still in flight, and handleKeydown (:1616-1624) calls handleSubmit() directly — bypassing PendingButton''s repeat-click suppression — submitting a half-reset form. The plan asserts the ordering is load-bearing without specifying this. Also: defineExpose''s submit (:1839) calls bare handleSubmit(), so the mode parameter must be genuinely defaulted to ''navigate'' or the inline-create contract breaks.'
severity: significant
status: addressed
resolution: >-
  Plan step 3 now requires the reset to be awaited INSIDE handleSubmit's try, keeping saving true across it, and requires mode to be defaulted to 'navigate' so defineExpose's bare submit() still works.
---
