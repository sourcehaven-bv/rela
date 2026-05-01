---
id: RR-B642B
type: review-response
title: pickInRelationPicker overlaps with existing addRelation helper
finding: e2e/pages/form.page.ts:286-290 (pickInRelationPicker) overlaps conceptually with FormPage.addRelation at lines 99-114 (search + click .dropdown-item). Two helpers, two selector strategies, neither aware of the other. At minimum cross-reference; better, fold one into the other.
severity: significant
resolution: Refactored FormPage.addRelation to delegate to pickInRelationPicker for the picker path (legacy <select> branch retained). Both helpers now share the same combobox-scoped selector strategy. Existing forms.spec.ts caller still passes.
status: addressed
---
