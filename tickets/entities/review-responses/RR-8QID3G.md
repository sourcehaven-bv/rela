---
id: RR-8QID3G
type: review-response
title: 'Renderability gate was dead code: could capture a blank form or eat the timeout'
finding: The gate did WaitVisible(#field-title) then checked for a toast-error. But the SPA renders empty schema-driven fields even on load failure, so WaitVisible can pass while the entity didn't load; the error toast is shown asynchronously, so the check races it and can miss — capturing a blank form as if real (the fail-OPEN hole the spike hit). Worse, if the title field never appears, WaitVisible runs out the full 30s and the author gets a useless 'context deadline exceeded'.
severity: critical
resolution: 'The form root now stamps data-testid="form-state-{pending|loaded|error}" off loadEntity''s actual outcome (DynamicForm.vue). The gate uses chromedp.Poll to race load vs error in one predicate: a load failure short-circuits with a clear ''entity failed to load'' message; success proceeds. No dependency on which schema fields render, no timing-sensitive toast, no timeout-as-error-signal. This also dissolved the hardcoded-''title''-anchor issue (S1). Regression test TestCapture_UnrenderableEntity_FailsLoud asserts a missing entity fails loud in ~3s, not 30s.'
status: addressed
---
