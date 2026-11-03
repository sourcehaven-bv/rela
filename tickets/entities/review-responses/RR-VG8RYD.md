---
id: RR-VG8RYD
type: review-response
title: 'Simpler default: fold in Go over the gated header path'
finding: Native SQL adds builders and divergence risk; a Go fold over FilterHeaders gives parity with a GET.
severity: significant
reason: The brief requires database backends to aggregate in SQL. Divergence risk is handled by the conformance suite comparing each native answer to store.RollupGeneric on the same store; the generic fold remains the fs/memory path and the SQLite unsafe-name fallback.
status: wont-fix
---
