---
id: RR-VFPYX
type: review-response
title: No check that entityId prefix matches entity_type
finding: 'Button is gated only on docConfig.entity_type. If a doc is misconfigured with entity_type: ticket and the user navigated via /document/foo/CAT-001, clicking Edit pushes /form/edit_ticket/CAT-001 and DynamicForm.loadEntity will fail. EntityDetail doesn''t have this issue because the route encodes the type. The plan should pick: (a) accept it (form view surfaces the load error), or (b) gate additionally on the entityId prefix matching the type''s id_prefix (same logic at DynamicForm.vue:78-88). Either is fine; pick one explicitly.'
severity: minor
reason: 'Accepted: a misconfigured edit.form whose entity type doesn''t match the doc''s entity_type is a config error that surfaces via DynamicForm''s existing load-error path. A cross-config consistency check (cfg.Forms[edit.form].EntityType == doc.EntityType) is a sharper invariant and belongs in a broader pass over kanban.edit_form / list.edit_form / document.edit.form together — deferred to its own ticket.'
status: wont-fix
---
