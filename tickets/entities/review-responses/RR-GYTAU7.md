---
id: RR-GYTAU7
type: review-response
title: One invalid path rejects the whole PUT (search q)
finding: 'Code review: EntityList adds q while the search box has text. q was not in the list allowlist. The whole PUT got 400 and every other subscription stopped updating.'
severity: significant
resolution: 'q is allowed on list paths. TestParseLivePath has a case with every ListParams key plus q. Per-entry errors were not added: the client builds every path from the same params it fetches with.'
status: addressed
---
