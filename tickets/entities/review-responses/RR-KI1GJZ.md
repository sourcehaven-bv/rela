---
id: RR-KI1GJZ
type: review-response
title: api-reference busy paragraph omits when no channel change repeats a busy list
finding: 'docs/data-entry/api-reference.md:141-149 does not say that a busy list whose page is empty, or whose busy read was not served (unknown type, realtime off), is not repeated by channel changes or by the safety check, only by entity changes and reloads. Fix: add one sentence.'
severity: nit
resolution: 'Fixed: the ?busy= paragraph says that a list with an empty page, or whose _busy was not answered for a reason other than a store error, is repeated only after an entity change or a reload, not on channel changes or by the safety check.'
status: addressed
---
