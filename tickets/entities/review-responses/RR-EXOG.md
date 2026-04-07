---
id: RR-EXOG
type: review-response
title: 'S3: Referer fallback semantics and curl breakage explicitly documented'
finding: Reviewer pointed out that the Referer fallback has subtle browser-policy interactions and that rejecting requests with no Origin AND no Referer breaks legitimate non-browser clients. The fallback path is a deliberate trade-off but was undocumented.
severity: minor
resolution: 'docs/security.md now has an explicit Calling the API from curl section showing the workaround (`curl -H ''Origin: ...''`), and the Troubleshooting section explains origin_missing. Trade-off is now visible to operators rather than hidden in the middleware code.'
status: addressed
---
