---
id: RR-BB01GI
type: review-response
title: Column gate is wider than the hash dedup, so false-dirty rows starve again
finding: 'Rows the SQL column gate marks dirty but the Go dedup skips stay candidates forever: a purge tombstone followed by an unchanged save (timestamp fallback), and numbers whose stored text differs from their canonical hashed form (uint64 above 2^53). A batch of either brings BUG-1DWMYO back.'
severity: significant
resolution: Replaced the column gate with a stored content_hash on live rows (pgstore migration 0020, sqlitedb v14). The sweep writes back the hash it computes, guarded by xmin (pg) or content equality (sqlite); a trigger clears it when a hashed column changes. The gate now compares exactly what the dedup compares. The backlog test seeds every row with uint64 max and covers the purge case; both fail against the column gate.
status: addressed
---
