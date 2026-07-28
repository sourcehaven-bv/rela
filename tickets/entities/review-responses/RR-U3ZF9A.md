---
id: RR-U3ZF9A
type: review-response
title: Explicit DynamicForm save sends the WHOLE formData as 'ours', not a delta — stale values resurrect under merge
finding: 'Step 1 adds If-Match to the explicit save at DynamicForm.vue:887, which sends properties: pruneWizardHidden(formData.value) — the ENTIRE form, every property loaded at loadEntity, not a delta (visibleWritablePropertiesForCommit short-circuits to {...formData.value} for isEdit). Today that is survivable: the server''s maps.Copy merges it over the live entity. Under the plan it becomes dangerous, because ''ours'' is a full snapshot of a form possibly open for an hour. Failure: Alice opens the form 09:00, leaves the tab open. Bob updates priority and assignee at 09:30. Alice changes only title and saves at 10:00 → 412. For priority: base should be Alice''s 09:00 value, ours = the same untouched value, theirs = Bob''s → ours===base → take theirs (correct) — but ONLY if a base was recorded. Per RR-VQQQ60, DynamicForm records NO base, so base=undefined and all three differ → conflicts on priority/assignee, fields Alice never touched. If the implementation instead resolves ambiguity toward ours, Alice''s hour-old values silently overwrite Bob — the original bug amplified from one field to the whole entity. FIX: either narrow the explicit save to a dirty-field delta before enabling If-Match, or define ours as formData diffed against a properly-recorded base (same dependency as RR-VQQQ60).'
severity: significant
status: open
---
