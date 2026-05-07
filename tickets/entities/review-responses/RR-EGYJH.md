---
id: RR-EGYJH
type: review-response
title: Helper extraction location unspecified
finding: Helper should live in frontend/src/utils/entityRoute.ts as an exported function (not script-local in CustomView.vue), so (a) it's unit-testable in isolation and (b) the inevitable 'now do EntityList too' refactor doesn't have to copy-extract first.
severity: minor
resolution: Helper goes in frontend/src/utils/entityRoute.ts as exported function. Importable by EntityList.vue, CustomView.vue, and any future consumers.
status: addressed
---
