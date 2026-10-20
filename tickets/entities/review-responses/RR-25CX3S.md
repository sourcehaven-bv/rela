---
id: RR-25CX3S
type: review-response
title: Composer datetime refuses years < 100 with a DST message
finding: channelValues.ts:67 new Date(y, ...) maps years 0-99 to 1900+y, so '0050-01-01T10:00' fails the round-trip check and reports 'does not exist in your time zone (the clocks change then)'. Use setFullYear(y) after construction, or refuse years outside 1000-9999 with a proper message. fromPropValue also drops sub-second precision, so an edit rewrites a stored value that had milliseconds. Fine if the server never stores them, but say so.
severity: nit
resolution: Datetime conversion uses setFullYear so years below 100 are kept, and the module header notes that the server stores whole seconds.
status: addressed
---
