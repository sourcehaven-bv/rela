---
id: RR-5KBP10S
type: review-response
title: Hidden date input announced beside Now
finding: The empty date input under the Now button kept its aria-label so a screen reader met two controls for one action.
severity: minor
resolution: The empty input is aria-hidden as well as out of the tab order. Test asserts both.
status: addressed
---
