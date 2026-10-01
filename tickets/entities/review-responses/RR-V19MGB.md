---
id: RR-V19MGB
type: review-response
title: Long single-word label widens its label column
finding: Without a fixed width a long single word sets the label's minimum width and shifts that row's value.
severity: minor
resolution: 'Label has min-width: 0 and overflow-wrap: anywhere, so a long word breaks inside the 120px column.'
status: addressed
---
