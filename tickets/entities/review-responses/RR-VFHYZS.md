---
id: RR-VFHYZS
type: review-response
title: SQLite EntityIDs can exceed the bind variable limit
finding: One placeholder per id; gantt subtrees are unbounded.
severity: significant
resolution: 'Plan: sqlitestore chunks EntityIDs relation reads; test above 1000 ids.'
status: addressed
---
