---
id: RR-3IMPOH2
type: review-response
title: Configure can widen a flow step's write bound past outside_edits locks
finding: '[security] internal/appbuild/stepforms.go, flowsteps.go inputNames, configedit allowlist. The step bound used to live in flows.yaml (locked in Configure); it is now the non-hidden fields of a data-entry.yaml form, which Configure may edit. A step write runs under flowedits.WithinRun, which exempts its own state''s outside_edits rules, so adding {property: budget} to a step form (or un-hiding it) lets a reviewer write budget in a state that freezes it. CheckTriggerACL only treats form fields as trigger locks and never judges the widening. Fix: refuse a data-entry.yaml edit that adds a property to the resolved bound of a form a flow state names (compare resolveStepForms before/after); narrowing stays allowed.'
severity: significant
resolution: 'CheckTriggerACL judges a data-entry.yaml-only edit by its step-form change (judgeFormEdit/stepFormChange): a form that adds a property the flow''s outside_edits lock in that step''s state is refused with a step_form problem on data-entry.yaml. Pinned by TestCheckTriggerACL_StepFormEdits.'
status: addressed
---
