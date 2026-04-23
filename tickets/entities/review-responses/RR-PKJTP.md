---
id: RR-PKJTP
type: review-response
title: patternMatches/matchPattern duplication is premature optimisation
finding: internal/frontendroutes/routes.go:91-137 has two near-identical match loops. Has is called at most once per rela.url call and once per rewritten href — tens to low hundreds per doc render. matchPattern's map alloc is already lazy. Delete patternMatches; implement Has as `_, ok := matchPattern(...); return ok`.
severity: minor
resolution: Removed duplicate matchPattern function; Has now simply calls `_, ok := Match(path)`. Map allocations were a non-concern at catalog size; the duplication was premature optimisation that cost readability.
status: addressed
---
