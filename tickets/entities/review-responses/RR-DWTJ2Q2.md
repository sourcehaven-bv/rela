---
id: RR-DWTJ2Q2
type: review-response
title: No test pins the detail-page routing wiring
finding: EntityDetail built the routing hint inline, so no test proved that a link property routes to the url widget on the detail page.
severity: minor
resolution: Added viewFieldRoutingHintFor, used by EntityDetail.fieldRowsFor, with a unit test.
status: addressed
---
