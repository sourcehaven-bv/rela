---
id: RR-VF1XC5E
type: review-response
title: carryProp sends a string list back into an object list
finding: After a reload turning a string list into an object list, an edit carried the old strings and was refused.
severity: minor
resolution: carryProp carries an object list only when every item is an object; unit test added.
status: addressed
---
