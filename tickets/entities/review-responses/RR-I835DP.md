---
id: RR-I835DP
type: review-response
title: GraphQuery re-runs recursive CTEs on every page
finding: Keyset pages re-evaluate the entity-closure CTE for every entity of the type on each page.
severity: significant
resolution: The page resume id now narrows the candidate-rooted closure seed (sqlBuilder.pageAfter, entityClosureSQL). Endpoint closures do not depend on the candidate and stay per statement; queries with Limit/Offset/OrderBy remain one statement.
status: addressed
---
