---
id: RR-V9TL16
type: review-response
title: The one-click required-property pre-check duplicates NormalizeProps and misreads null and []
finding: 'Approach 4 adds a check before `NormalizeProps` that "finds the first required property the preset leaves out". NormalizeProps treats a nil value and an empty list as absent (internal/realtimecfg/props.go:58-73). A key-presence pre-check would pass `feedback: ~` or `tags: []` and then fall back to the old message without the hint. It also changes the error order for a preset that has an undeclared property and a missing required one. Simpler and exact: always call `normalize(..., false)`, derive the left-out set from the result, and for a one-click button report the first required left-out name with the hint.'
severity: minor
resolution: ADDRESSED Every preset goes through one normalize(..., false) call; the left-out set and the one-click hint come from its result, so ~ and [] count as absent. Prop errors are reported before the missing required property (AC 2, Approach 4).
status: addressed
---
