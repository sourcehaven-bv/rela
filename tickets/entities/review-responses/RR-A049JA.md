---
id: RR-A049JA
type: review-response
title: buildAutoSaveRelationsBody still aborts every relation field when one ID is unresolvable
finding: 'DynamicForm.vue''s autosave and submit paths both abort ALL relation fields when any single ID has no resolvable type, because reshapeLegacyToModern returns one nullable record for the whole set. The fix makes the trigger rarer but not impossible -- it still fires at the 50-page cap and on any future consumer that populates pickerTypes partially. The blast radius is unchanged and the failure is still silent to the server: no PATCH at all, chips still on screen, edit lost on reload.'
severity: significant
reason: Deferred to TKT-QF41FL. Narrowing this is a behaviour change to the shared save path affecting every relation field on every form, and needs its own test matrix; the reviewer agreed the call not to bundle it was right. Filed rather than left to memory precisely because it is a known whole-form-abort on a shared save path.
status: deferred
---
