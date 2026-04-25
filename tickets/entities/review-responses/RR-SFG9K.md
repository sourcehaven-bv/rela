---
id: RR-SFG9K
type: review-response
title: getEditFormId fallback can land user on a create form in silent edit mode
finding: 'getEditFormId (frontend/src/types/config.ts:123-136) prefers mode:''edit'' but falls back to ANY form for the entity type. If only a create_ticket form is configured, the helper returns it. DynamicForm.vue:58 then flips to edit mode purely on !!entityId, which means: defaults are not initialized, templates are not loaded, the title says ''Edit Ticket'' even though YAML keyed it as create_ticket, and create-only field rules don''t apply. EntityDetail has the same latent bug, so this is a shared smell rather than a regression — but the plan does not call it out. Either tighten getEditFormId to not fall back to non-edit forms, or document the limitation explicitly so a future fix can find both call sites.'
severity: significant
resolution: 'Addressed by removing the getEditFormId auto-resolve entirely from this design. New approach: explicit per-document edit config (edit.form, edit.label) in data-entry.yaml, validated at config load against cfg.Forms. No magic = no fallback trap. The EntityDetail.vue smell is unchanged but no longer replicated at a second call site.'
status: addressed
---
