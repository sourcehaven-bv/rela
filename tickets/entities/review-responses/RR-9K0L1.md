---
id: RR-9K0L1
type: review-response
title: parsePropertiesArg returns (nil, true) for JSON 'null'
finding: internal/mcp/tools_helpers.go:74-93. json.Unmarshal([]byte("null"), &props) succeeds with err==nil and props==nil. parsePropertiesArg returns (nil, true) while every other path returns (nil, false) for missing/malformed. Both consumers happen to handle nil input defensively, so behavior is correct today — but it's a footgun for future callers. Either reject (return (nil, false)) after unmarshal yields a nil map, OR document the (nil, true) case loudly on the helper.
severity: significant
resolution: Fixed in tools_helpers.go:parsePropertiesArg — after json.Unmarshal of a string arg, if the resulting map is nil (i.e. the input was the JSON literal `null`), return (nil, false) so the caller treats it as missing/malformed. Added TestExtractPropertiesAllowNil_JSONNullArg covering the case.
status: addressed
---
