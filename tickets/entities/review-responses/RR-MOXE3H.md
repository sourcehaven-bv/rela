---
id: RR-MOXE3H
type: review-response
title: Unify the version-sweep literal and duplicated attribution helpers
finding: attributionValues and sweepAttribution were copied verbatim in both backends and the version-sweep string appeared in four places.
severity: minor
resolution: Hoisted to internal/store/attribution.go as AttributionColumns, SweptPrincipal and SweepPrincipalTool; both backends and storetest use them.
status: addressed
---
