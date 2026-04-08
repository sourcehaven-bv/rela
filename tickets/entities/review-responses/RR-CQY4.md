---
id: RR-CQY4
type: review-response
title: API_TO_UI_OPERATOR override needs explanatory comment
finding: 'filters.ts API_TO_UI_OPERATOR pins eq to ''='' with an explicit override after the spread. The current comment says ''eq is pinned to = (not ==) since OPERATOR_MAP has two UI forms for it''. That''s accurate but doesn''t explain WHY the spread alone is wrong: Object.fromEntries last-write-wins on duplicate keys, and OPERATOR_MAP has both ''='' → ''eq'' and ''=='' → ''eq'', so without the override the inverse would map eq → ''==''. Expand the comment so the next maintainer doesn''t need a debugger.'
severity: nit
resolution: 'filters.ts API_TO_UI_OPERATOR comment expanded to explain that Object.fromEntries does last-write-wins on duplicate API keys, that OPERATOR_MAP iteration yields eq → ''=='' as the final entry, and that the explicit eq: ''='' override after the spread is what makes fromApiOperator(''eq'') return the shorter canonical form. Future maintainers don''t need a debugger to understand the pinning.'
status: addressed
---
