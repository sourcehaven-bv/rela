---
id: RR-RV7WHL
type: review-response
title: loadTemplates() clobbers kept values, discards the user's template pill, and refetches per record
finding: 'The plan orders capture → clear → initializeDefaults → re-apply kept → loadTemplates. But loadTemplates (DynamicForm.vue:680-693) unconditionally re-fetches over HTTP, resets selectedTemplate to templates[0], and applyTemplate writes formData — so a template naming a keep_on_add_another property clobbers the carried-over value, inverting AC-4b. It also discards an explicitly chosen template pill and costs a round trip per record. Fix: re-apply kept values AFTER templates, and call applyTemplate against the already-loaded templates ref rather than refetching.'
severity: significant
status: addressed
resolution: >-
  Plan step 1 no longer calls loadTemplates(). It calls applyTemplate against the already-loaded templates ref using the user's selectedTemplate, and re-applies kept values AFTER the template so a template cannot clobber them. Final order pinned in the plan.
---
