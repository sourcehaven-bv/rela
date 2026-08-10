---
id: RR-IRV2WJ
type: review-response
title: int binder must preserve string->int coercion (pinned test)
finding: 'resolver_test.go:358 TestResolver_OffTypeProperty_CoercesNotFails stores priority: ''5'' (string) for an integer field and asserts entity.priority == 5 passes. Post-swap the int binder (integer->NewInt) must keep the permissive coercion coerceNumber has (bindings.go:185-198): int, int64, float64-with-integrality-check, AND string (strconv.ParseInt). If someone ''cleans up'' the binder to accept only int/int64 (since IntType ''should'' see integers), this test breaks and real entities with hand-stored ''5'' flip to Nil->deny. FIX: AC1 explicitly requires int binder parity with coerceNumber''s accepted shapes; keep TestResolver_OffTypeProperty_CoercesNotFails green; add time.Time-for-date sibling.'
severity: significant
resolution: 'coerceInt (affordances/bindings.go) preserves the permissive coercion: int, int64, integral-float64, and string (strconv.ParseInt) all bind; fractional float / non-numeric string -> Nil (no silent truncation). TestResolver_OffTypeProperty_CoercesNotFails (string ''5'' on int field) stays green; TestResolver_IntegerWhen_NumericOrdering adds ''10''-string and int64 cases + numeric ordering (100 > 9 true, not lexicographic).'
status: addressed
---
