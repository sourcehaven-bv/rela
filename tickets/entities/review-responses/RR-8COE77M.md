---
id: RR-8COE77M
type: review-response
title: Picker with several types filters client-side
finding: EntityPickerModal with types.length > 1 sends no type and filters resp.data locally (EntityPickerModal.vue:108-115); limit=50 could leave zero results.
severity: significant
resolution: 'Plan changed: the picker sends limit only when it searches at most one type.'
status: addressed
---
