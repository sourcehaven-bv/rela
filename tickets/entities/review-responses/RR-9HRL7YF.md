---
id: RR-9HRL7YF
type: review-response
title: QueryExecModeExec sends parameter types from Go types
finding: Predicate and field-grant arguments had not run in exec mode before; a numeric argument could be typed differently.
severity: minor
resolution: The full pgstore suite including RunVisibleSearchTests and RunFieldScopeSearchTests and the faced-world conformance tests passes with the store running every search in this mode.
status: addressed
---
