---
id: RR-KFQD4
type: review-response
title: Whitespace test has fragile substring assertion
finding: 'loader_test.go uses strings.Contains(err.Error(), "whitespace"). Will match any error in the aggregated SchemaValidationError that happens to contain the word ''whitespace'' — including the unrelated WhitespacePropertyError. Test passes spuriously if the implementation regresses to ''use property-name whitespace check.'' Pin to the specific sentence: assert both ''display_property'' and ''whitespace'' substrings are present.'
severity: minor
resolution: TestParse_DisplayPropertyWhitespace now asserts the diagnostic contains 'display_property', 'whitespace', AND 'titel' (the actual property name). Pinned alongside RR-MPE9Y so a regression to a different validator's whitespace check would fail — not pass spuriously.
status: addressed
---
