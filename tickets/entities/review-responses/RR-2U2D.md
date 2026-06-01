---
id: RR-2U2D
type: review-response
title: 'Race: commit can DROP a user-filled field whose dry-run hasn''t resolved yet'
finding: 'DynamicForm.vue visibleWritablePropertiesForCommit() filters formData against stagedVisibleProps + writable, gated on stagedAffordancesReady. If the user types in a field then Cmd+Enter before the 400ms debounce fires, the staged dry-run hasn''t yet learned that field is visible/writable (its key isn''t in stagedVisibleProps and there''s no _fields entry), so the value is silently dropped. The server then applies its own default; user thinks they set it. Worse: a metamodel-default field the user never even saw (because some OTHER unrelated field is hidden but the form''s data still carried this one''s default) also gets sent today. Fix options: (a) before commit, flush any pending dry-run (await refresh if a timer is armed) and apply the result, (b) track a userTouched Set and never drop touched keys (also closer to autosave semantics).'
severity: significant
resolution: 'Added userTouched Set populated by updateField in create mode. visibleWritablePropertiesForCommit always preserves user-touched keys regardless of staged dry-run state, so a key the user just typed into can''t be silently dropped during the debounce window. Safety: the server''s affordance gate (BUG-Q60V) still rejects a touched key the policy denies, so the user gets a clear 403 with rule_id rather than a silent drop. Also fixes the latent issue of sending stale metamodel defaults the user never touched (only touched + explicitly visible-writable keys ship).'
status: addressed
---
