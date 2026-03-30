---
finding: Default case returns empty string for slices, maps, custom types. This silently drops data like []string from WithList() when writing via ProjectBuilder.
id: RR-hyg8
resolution: Added []string case to toString() that formats slices as YAML inline arrays [a, b, c].
severity: significant
status: addressed
title: toString() silently returns empty for unsupported types
type: review-response
---
