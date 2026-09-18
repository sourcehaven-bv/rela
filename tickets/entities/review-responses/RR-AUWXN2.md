---
id: RR-AUWXN2
type: review-response
title: Per-job module cache would blow the 10GB quota and evict itself
finding: 'The first cut cached ~/.cache/go-build and ~/go/pkg/mod together under one per-job key, across 15 Go jobs plus 6 cross-compile matrix legs. Measured against the live repo: 10 populated per-job caches totalled 7.04GB (avg 0.70GB), projecting ~12.6GB against GitHub''s 10GB per-repo quota. Eviction is LRU and repo-wide, so the entries would continuously evict each other and the previous generation the restore-keys depend on, leaving CI colder than before the change. The symptom is invisible on a green run: it just gets slower over weeks.'
severity: critical
resolution: Split the two paths by their sharing properties. The module cache is a function of go.sum alone and therefore byte-identical in every job, so it went back to setup-go's own (correctly go.sum-keyed) shared entry — eliminating ~14 redundant copies. Only the build cache, which genuinely differs per job by package set and build tags, keeps a per-job key. Also purged the 13 stale caches from both the old and first-cut schemes; repo usage went from 10.18GB to well under quota.
status: addressed
---
