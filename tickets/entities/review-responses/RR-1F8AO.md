---
id: RR-1F8AO
type: review-response
title: edit.form unknown-form error lacks the typo suggestion that list.edit_form provides
finding: validate.go's edit.form unknown-form check follows the kanban.edit_form path (no suggestion) instead of the list.edit_form path which uses suggestForm() for 'did you mean' hints. The plan said 'same wording shape as list/kanban edit_form errors' so technically consistent with kanban, but inconsistent with list. Cheap to add the suggestion call for parity with the more helpful error.
severity: significant
resolution: Wrapped the unknown-form check in suggestForm() like list.edit_form does; new error format `edit.form references unknown form %q (did you mean %q?)`. Added TestValidateConfig_DocumentsEditFormSuggestion to verify the typo path.
status: addressed
---
