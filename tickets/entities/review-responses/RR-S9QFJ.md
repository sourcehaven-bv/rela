---
id: RR-S9QFJ
type: review-response
title: filterEmptyStrings and filterNilAndEmpty duplicate ~90% of code
finding: tools_helpers.go:97-136. Two near-identical helpers differ only in whether they skip nil. A single helper parameterized by a 'keepNil bool' (or a predicate) would remove the duplication and prevent the next 'filter X also' change from needing two edits.
severity: minor
resolution: Consolidated filterNilAndEmpty and filterEmptyStrings into a single filterProperties(props, keepNil bool) helper. Both extractProperties and extractPropertiesAllowNil now call this single helper with the appropriate flag. Removed the duplicate code.
status: addressed
---
