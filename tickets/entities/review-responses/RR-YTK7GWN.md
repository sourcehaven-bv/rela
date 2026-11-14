---
id: RR-YTK7GWN
type: review-response
title: Scope and hook values skipped type conversion
finding: A scope constant or hook text reached an integer property as a string.
severity: minor
resolution: Parse checks scope values convert; planCreate stores the converted value; hookScalar parses text on integer and boolean properties (TestScopeAndHookValuesAreConverted).
status: addressed
---
