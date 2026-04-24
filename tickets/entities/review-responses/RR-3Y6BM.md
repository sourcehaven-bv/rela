---
id: RR-3Y6BM
type: review-response
title: Rewriter/cache contract undocumented
finding: api_v1.go runs RewriteDocumentLinks AFTER fetching from disk cache; GetCached returns HTML with no baked-in return_to. Widening the rewriter to all internal paths does not change that contract, but the plan never affirms it. A future optimisation pushing the rewrite into doRender would poison the disk cache with the first visitor's return_to, served to all subsequent viewers. No test guards this invariant today.
severity: critical
resolution: Added AC9 (cache invariance test) + explicit invariant in Security Considerations + will add rewriter comment at implementation time. Plan explicitly states the rewriter runs post-cache in api_v1.go, never inside doRender.
status: addressed
---
