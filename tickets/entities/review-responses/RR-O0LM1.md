---
id: RR-O0LM1
type: review-response
title: int64-max literal coercion relies on impl-defined float->int
finding: 'coerceIntLiteral (walk.go) checks integrality via n.v != float64(int64(n.v)) then does int64(n.v). Number literals are float64, so a literal >= 2^63 (e.g. int64 max 9223372036854775807, stored as 9223372036854775808.0) can pass the integrality guard and silently become a wrong/saturated value. Go''s out-of-range float->int conversion is implementation-specific per spec (not guaranteed to saturate on every arch). For an ACL boundary, arch-dependent integer comparison is unacceptable. Fix: parse integer literals from the AST source token as int64 directly, OR explicitly reject |n| >= 2^53 at compile with a clear error.'
severity: significant
resolution: coerceIntLiteral (walk.go) now rejects any integer literal with magnitude >= 2^53 (maxExactIntLiteral) at COMPILE time with a clear error ('too large to compare exactly (must be within ±2^53)'), before the impl-defined float->int64 conversion can run. 2^53 is the largest magnitude where every integer is exactly representable in float64, so within the bound the round-trip is exact and arch-independent. Real int-property comparisons are far below this. Verified by typed_coercion_test.go 'int literal beyond 2^53'.
status: addressed
---
