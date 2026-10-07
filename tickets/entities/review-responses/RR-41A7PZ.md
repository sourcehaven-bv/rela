---
id: RR-41A7PZ
type: review-response
title: Tx-context marking is caller-dependent and mostly untested
finding: Only DeleteEntity is covered; several Tx sites are unmarked and nothing stops a future caller reaching tag writes.
severity: significant
resolution: archguard txctx_test.go fails on an unmarked Tx callback; seven sites marked; allowlist limited to perfseed and sqlitestore internals.
status: addressed
---
