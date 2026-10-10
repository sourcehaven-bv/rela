---
id: RR-U7R63P
type: review-response
title: Non-finite floats lost their literal text
finding: A .inf or .nan value was re-encoded in a different spelling, so an unedited line changed.
severity: minor
resolution: tree.go keeps the literal text of inf and NaN floats. TestFromYAML_NonFiniteFloatsStayText.
status: addressed
---
