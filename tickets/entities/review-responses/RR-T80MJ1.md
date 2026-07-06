---
id: RR-T80MJ1
type: review-response
title: bleve textAnalyzer allocated a fresh registry.NewCache per MatchedFields call
finding: fieldmatch.go built a new registry cache and re-looked-up the standard analyzer once per surviving hit per query — avoidable hot-path allocation.
severity: nit
resolution: Cached via package-level sync.OnceValue(cachedStandardAnalyzer); the standard analyzer is stateless and goroutine-safe.
status: addressed
---
