---
id: RR-6ME8MR
type: review-response
title: Scoped entries are not store-paged
finding: listPage takes the store-paged path only when no condition and no query scope apply (internal/dataentry/api_v1.go:393). Any scoped entry, including one that inherits a type default scope, reads every row the scope's pushable conjuncts match, re-checks the scope in Go, sorts in Go and then slices. The plan claimed one paged, pushed-down, indexed query per refetch.
severity: significant
resolution: 'Risk section corrected: a sidebar entry costs what the equivalent scoped list page costs today. The scope''s store-safe conjuncts still narrow the read. Refetches are deduplicated through the Pinia Colada query cache and the server batches entity:changed per type every 200 ms. Implementation will measure a scoped entry on the postgres perf seed with rela-server -verbose; a bad result goes back to the user before continuing.'
status: addressed
---
