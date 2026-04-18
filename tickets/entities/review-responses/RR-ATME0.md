---
id: RR-ATME0
type: review-response
title: metamodel/loader.go prealloc capacity could be cleaner
finding: 'In internal/metamodel/loader.go:207 the expression (len(valueNode.Content)-1)/2+1 over-allocates by 1 for empty mappings. Cleaner: make([]string, 0, len(valueNode.Content)/2).'
severity: nit
reason: Nit-level allocation formula. Both forms are correct; the difference is one slot of capacity for an empty mapping. Current form was generated directly by the prealloc linter's suggestion. Not worth another commit.
status: wont-fix
---
