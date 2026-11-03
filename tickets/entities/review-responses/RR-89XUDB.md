---
id: RR-89XUDB
type: review-response
title: Duplicate check does not normalise
finding: AC 1 compares items exactly after trimming. `e` + U+0301 and U+00E9 render identically and pass; so do `Yes`/`yes` (arguably fine). Recommend NFC-normalising each item before storing and comparing (golang.org/x/text/unicode/norm), with a test.
severity: minor
resolution: Each item is NFC-normalized with golang.org/x/text/unicode/norm (already a dependency) before storing and comparing, with a duplicate test for U+00E9 against e plus U+0301.
status: addressed
---
