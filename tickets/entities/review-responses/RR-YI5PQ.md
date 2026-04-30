---
id: RR-YI5PQ
type: review-response
title: valueOptions partial-coverage for cross-type non-uniform enums
finding: For properties that have enum values on SOME entity types but not others, valueOptions unions only the enum values from the types where it is enum, silently hiding the fact that other types accept arbitrary strings. The dropdown then constrains the user to a subset of legal values.
severity: significant
reason: Cross-type non-uniform enums (a property that's enum on type A and free-string on type B) are an edge case for SearchView. The current code unions only the enum values; the user may need to type a value that the dropdown won't show. Left as a known limitation — fixing requires broader UX rework of search-mode value entry. Documented in code comment in valueOptions.
status: deferred
---
