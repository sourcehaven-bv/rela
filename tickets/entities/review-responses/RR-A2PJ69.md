---
id: RR-A2PJ69
type: review-response
title: Stale doc on resolveListRelations; budget counts store calls; test mutates app metamodel
finding: resolveListRelations still said it loads each row's edges once. The budget is in store calls not SQL round trips on pgstore. The budget test mutates app.Meta() in place without saying why that is safe.
severity: nit
resolution: Updated the resolveListRelations doc. The budget constant's comment says the count is in store calls. The test says budgetMeta and budgetConfig are fresh per app.
status: addressed
---
