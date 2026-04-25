---
id: RR-9CW5N
type: review-response
title: DisplayTitle does not stringify non-strings — plan rationale is wrong
finding: 'Plan claims runtime stringifies non-string values via fmt.Sprintf and only falls back to ID for empty. False — entity_def.go:92-101 does val.(string) and falls THROUGH to ID on type-assertion failure. Author setting display_property: status (enum) would have every entity render as its ID instead of e.g. ''open'' / ''in-progress''. Worse than today''s autoderivation.'
severity: critical
resolution: 'Pick fix (a): stringify non-string values via fmt.Sprintf("%v", val) inside DisplayTitle, with ID fallback when the stringified result is empty. Add test case display_property points at an enum. Keeps the ''trust the author'' principle while making the runtime actually do what the plan claimed.'
status: addressed
---
