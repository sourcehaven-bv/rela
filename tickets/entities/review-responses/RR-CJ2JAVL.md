---
id: RR-CJ2JAVL
type: review-response
title: 'Security: pending relation reload queries the from x to product'
finding: relationViews lists all edges among the froms and tos of the pending relations which on a dense type can exceed the wanted keys.
severity: minor
resolution: The scan stops once every wanted key is found. The query only returns existing edges between at most MaxRows heads and tails and runs only for unfilled relation elements (backfill).
status: addressed
---
