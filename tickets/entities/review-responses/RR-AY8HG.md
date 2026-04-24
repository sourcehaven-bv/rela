---
id: RR-AY8HG
type: review-response
title: .back-btn class collision + App.vue lint
finding: 'SearchView:557 already uses .back-btn for its filter-picker internal affordance. Plan proposes lifting both .scope-nav-btn AND .back-btn from EntityDetail to App.vue. The .back-btn collision is real: what gets lifted vs what stays scoped? Also: App.vue already trips the max-lines: 500 lint warning; piling styles there pushes it further. Resolution: rename the new component''s class (use .scope-nav-btn only, no new .back-btn), and put shared styles in src/styles/back-button.css imported from main.ts, not App.vue.'
severity: significant
resolution: 'Only .scope-nav-btn class is lifted; no new .back-btn (avoids collision with SearchView''s scoped filter-picker .back-btn). Shared styles live in src/styles/back-button.css imported from main.ts, not App.vue (dodges the max-lines: 500 warning).'
status: addressed
---
