---
id: RR-ZEZMQ9A
type: review-response
title: JSON subset semantics undocumented; float tolerance
finding: An empty want table matches any container, arrays compare at exact length, and a relative float tolerance made large ids compare loosely.
severity: minor
resolution: Numbers compare exactly (both sides are float64). The empty-table and array rules are documented in jsonSubset and the module godoc; Lua cannot tell an empty object from an empty array.
status: addressed
---
