---
id: RR-HDCZ6
type: review-response
title: Hot-path concern overstated
finding: Reviewer noted GetPrimaryProperty is O(properties) but bounded by request-time list size; the new short-circuit is strictly an improvement on the override path. No caching needed today.
severity: nit
resolution: No action; closed for the record. Caching becomes load-bearing only if/when display_format extension lands.
status: addressed
---
