---
id: RR-YNWR7
type: review-response
title: JSON pointer escaping uses %q (Go-quoted) instead of RFC 6901
finding: |-
    Plan says relation-type names wrapped with %q in error pointers. %q is Go quoted-string — escapes for Go literals, not JSON pointer (RFC 6901: / → ~1, ~ → ~0). Relation type 'foo/bar' produces invalid JSON pointer; clients parsing it mis-route. Recommendation: add `jsonPointerEscape(s string) string` helper applying RFC 6901, or rename the field from 'JSON pointer' to 'structured error path' if %q semantics are intentional.

    From design-review: F6.
severity: significant
resolution: Plan Layer 1 specifies a `jsonPointerEscape(s string) string` helper in internal/dataentry/ applying RFC 6901 escaping (~ → ~0, / → ~1). Used uniformly for relation-type names and meta keys in error pointers. Documented as RFC 6901 in api-reference.md.
status: addressed
---
