---
id: RR-F8GQHND
type: review-response
title: Conversion to store.Cond belongs in queryplan
finding: 'Design review: do not write a second recursive LoweredCond converter in desktopnotify.'
severity: minor
resolution: Added queryplan.LoweredCond.PropCond, used by desktopnotify for both paths; TestLoweredCond_PropCond.
status: addressed
---
