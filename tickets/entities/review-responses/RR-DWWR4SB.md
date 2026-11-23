---
id: RR-DWWR4SB
type: review-response
title: A step whose form cannot be resolved completes as an empty step
finding: 'flowsteps.go inputNames(props, nil) accepts {} when x.Form is set but unresolved, and FlowStepForm.vue enables Complete when schema.getForm returns undefined. An approval step can pass without its data when the form is missing or stale. Fix: server refuses a step whose named form does not resolve; SPA shows an error and disables Complete when waiting.form names no known form. Test both.'
severity: significant
resolution: An unusable form is an errStepForm (409) on complete. The run view sets waiting.form_problem and offers no Complete. The SPA shows the problem and hides Complete; it does so too when the schema store lacks the form. Pinned by TestFlowControl_StepFormReadPerStep and FlowStepForm.test.ts.
status: addressed
---
