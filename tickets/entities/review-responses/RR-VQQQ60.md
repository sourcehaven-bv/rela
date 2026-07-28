---
id: RR-VQQQ60
type: review-response
title: DynamicForm never seeds a merge base — lastSeenServer is empty on the main edit form
finding: 'The plan''s foundation ("lastSeenServer/lastSeenContent are ALREADY the correct merge base — no new state required") is FALSE for the primary edit surface. useAutoSave seeds the base only from opts.initialServerSnapshot (useAutoSave.ts:254-255) or an explicit recordServerSnapshot call (:244). Verified: SectionEditForm.vue:95 passes initialServerSnapshot and EntityDetail.vue:348 calls recordServerSnapshot, but DynamicForm.vue — the main entity edit form and the explicit-save target of Step 1 — does NEITHER. loadEntity() assigns formData/content directly without handing the entity to useAutoSave. So lastSeenServer={} and lastSeenContent='''' until the first PATCH response. A three-way merge with base='''' degenerates: diff3 sees ''both sides added the entire document'' → whole-file conflict on the FIRST 412, and properties get base=undefined for every key. AC3 (disjoint fields auto-resolve, no UI) fails on every DynamicForm session. FIX: wire initialServerSnapshot/recordServerSnapshot into DynamicForm, AND add an explicit baseRecorded sentinel — undefined is ambiguous between ''never seen'' and ''genuinely absent server-side''; the merge must refuse to run and fall back to current behavior when no base was recorded.'
severity: critical
status: open
---
