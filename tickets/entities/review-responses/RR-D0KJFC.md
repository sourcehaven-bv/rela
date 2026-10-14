---
id: RR-D0KJFC
type: review-response
title: Export Mode methods but not the mode constants
finding: Exported constants let dataentry rebuild switches on the mode.
severity: minor
resolution: Constants are unexported; Mode has String() and the table tests live in package access.
status: addressed
---
