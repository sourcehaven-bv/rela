---
id: RR-T7O03SU
type: review-response
title: Unneeded readonly cast in FlowStepForm.vue
finding: 'FlowStepForm.vue: (f as { readonly?: boolean }) is unnecessary; FormFieldOrRelation already has readonly.'
severity: nit
resolution: Removed the cast; the field type already has readonly.
status: addressed
---
