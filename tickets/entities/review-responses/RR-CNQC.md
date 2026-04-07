---
id: RR-CNQC
type: review-response
title: Test coverage gaps for edge cases
finding: 'Missing tests for: date vs non-date mismatch, numeric vs non-numeric, list-valued properties, nil/missing property with lt/gte, gt/gte with numerics/strings, timezone behavior (needs clock injection). Existing tests check len() not specific IDs.'
severity: significant
resolution: 'Added tests: TestV1FilteringTypeMismatch (date vs non-date), TestV1FilteringMissingProperty (no property), TestV1FilteringInWithVariableTokens (in operator), TestCompareValues_TypeMismatch (cross-type detection), TestCompareOrdered_UnknownOperator. Existing tests now use a runListFilter helper and assert specific IDs instead of counts.'
status: addressed
---
