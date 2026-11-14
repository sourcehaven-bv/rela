---
id: RR-8RICWBV
type: review-response
title: Command payload doc overstated the list scope
finding: visibleListRows doc said the rows come from the read the list uses, but query_scope and page scope are not applied.
severity: minor
resolution: Doc now says the read covers the whole type and that the payload never applied the list scope\; the ACL gate keeps it safe.
status: addressed
---
