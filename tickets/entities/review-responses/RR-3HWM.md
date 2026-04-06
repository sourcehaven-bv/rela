---
id: RR-3HWM
type: review-response
title: Alignment strings should be extracted as constants
finding: The alignment strings 'left', 'right', 'center', 'none' appear in both extractTableData and renderTableNode. Should be constants to avoid round-trip breakage if one side is changed.
severity: minor
resolution: Extracted alignment strings as package constants (alignLeft, alignRight, alignCenter, alignNone) used in both extractTableData and renderTableNode.
status: addressed
---
