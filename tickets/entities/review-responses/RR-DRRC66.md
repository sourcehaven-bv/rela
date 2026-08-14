---
id: RR-DRRC66
type: review-response
title: Visibility is computed from formData so the seam still mutates before deciding
severity: critical
resolution: 'Approach step 1 rewritten: bindings-parameterised evalCond/activeKeys in useFormWizard give pure proposedBindings/wouldHide so the hypothetical is evaluable with no mutation. useFormWizard.ts added to Files to Modify.'
status: addressed
finding: 'Addressed in PLAN-6X0Y7W Approach step 1: hypothetical evaluation via bindings-parameterised evalCond/activeKeys, yielding pure proposedBindings/wouldHide with no mutation. useFormWizard.ts added to Files to Modify.'
---
