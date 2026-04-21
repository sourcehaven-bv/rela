---
id: RR-BUJU2
type: review-response
title: degree[source] accumulated but never read
finding: '`classifyRenderings` writes `degree[p.source] += len(p.to)` but only reads `degree[t]` for targets. Dead store.'
severity: nit
resolution: Classifier rewrite removed the dead source-accumulation entirely. New inDegree map only counts target-side edges (the only direction ever read).
status: addressed
---
