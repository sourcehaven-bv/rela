---
id: RR-E58BDB
type: review-response
title: Non-template values rendered with fmt.Fprint
finding: Lists render as [a b] and dates in Go's default format; value != nil differs from the IsEmptyValue check above it.
severity: nit
resolution: 'Non-template values now render through taskItemValueText: dates as YYYY-MM-DD and lists joined with commas.'
status: addressed
---
