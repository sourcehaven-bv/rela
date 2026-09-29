---
id: RR-AWIMJ3
type: review-response
title: PermitsReadMany failure branch untested
finding: The gate-error case set only faceErr for every type; the PermitsReadMany error branch and per-type isolation were not exercised.
severity: minor
resolution: Added TestResolver_ResolveHeadersRowGateErrorHidesOnlyItsType.
status: addressed
---
