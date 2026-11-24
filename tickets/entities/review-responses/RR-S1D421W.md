---
id: RR-S1D421W
type: review-response
title: Move trusted the client to send manual order flag
finding: A move without manual_order=1 got 404 and one with it judged against client order.
severity: minor
resolution: The move route sets manual_order=1 itself.
status: addressed
---
