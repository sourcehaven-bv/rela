---
id: RR-AEHM5
type: review-response
title: De Morgan rewrite of isHex-style predicates makes code harder to read
finding: In internal/attachment/hash.go:81 and several test files, the original positive-predicate-negated-once form (!((c >= '0' && c <= '9') || ...)) reads more clearly than the v2-autofix form ((c < '0' || c > '9') && ...). Semantically identical via De Morgan. Might be clearer to keep original form with //nolint:staticcheck // clearer as positive predicate.
severity: nit
reason: Nit-level readability preference. The staticcheck QF1001 rewrite is semantically identical via De Morgan. Reverting it and adding //nolint comments would trade one form of noise for another; the migrated code compiles and passes tests. Not worth churning the files again.
status: wont-fix
---
