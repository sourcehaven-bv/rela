---
id: RR-EKUKEW
type: review-response
title: List read resolves open suggestions twice; resolve cost unbounded per request
finding: Acceptable re-ran locate for every open suggestion, doubling resolve work; phase 3 adds a screening pass per anchor. 500 cross-block comments on a large body of short paragraphs cost minutes of CPU per list read.
severity: minor
resolution: Body memoizes locate per anchor so each comment is resolved once per request. A per-request resolve budget is a separate concern (phase 2 already carried most of the cost) and is tracked in TKT-D4EEYS.
status: addressed
---
