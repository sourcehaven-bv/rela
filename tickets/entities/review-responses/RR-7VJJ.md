---
id: RR-7VJJ
type: review-response
title: Multi-statement source must reject, not silently take first
finding: 'Plan says ''length != 1 is a compile error'' for return-statement Exprs, but `return true; return false` becomes TWO statements after the `return` prepend; the plan extracts the first ReturnStmt and apparently discards the rest. AC2 has no multi_statement.lua reject case. Add the contract: reject when len(chunk) != 1 OR when the single stmt is not a ReturnStmt OR when len(ReturnStmt.Exprs) != 1. Add three reject corpus files: multi_statement.lua, multi_return_value.lua, non_return_stmt.lua.'
severity: significant
resolution: 'Source-acceptance contract spelled out in plan: reject when len(chunk) != 1, when chunk[0] is not *ast.ReturnStmt, or when len(returnStmt.Exprs) != 1. AC2 reject corpus adds multi_statement.lua, multi_return_value.lua, non_return_stmt.lua.'
status: addressed
---
