---
id: RR-OOIAHIZ
type: review-response
title: "A revision reload drops extra pages"
finding: "After an action the Changes and Mentioned by lists read their first page again, so pages loaded with Show more go away."
severity: minor
reason: "The newest entries are on the first page, which is what an action changes; keeping stale later pages would need a merge by sequence that the panels do not have. Show more loads them again."
status: wont-fix
---
