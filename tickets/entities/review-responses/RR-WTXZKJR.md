---
id: RR-WTXZKJR
type: review-response
title: Unset value disagrees with write path
finding: Unset machine property reads as initial on write but Performable returns nil.
severity: significant
resolution: Toggle reads an unset machine property as the machine entry value and an unset bool or enum as off; tested.
status: addressed
---
