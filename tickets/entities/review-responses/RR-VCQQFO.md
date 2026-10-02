---
id: RR-VCQQFO
type: review-response
title: Date cells get the Go time.Time default format on the fs backend
finding: fs decodes an unquoted YAML date to time.Time and GetAttributeString formats it as 2026-09-26 00:00:00 +0000 UTC, which DateWidget parses unreliably (a day early west of UTC).
severity: significant
resolution: propertyToStrings(v, propType) writes time.Time as ISO (DateOnly for date, RFC3339 otherwise) and fillPropertyCell uses it. sections_test.go covers date, datetime and a list of dates.
status: addressed
---
