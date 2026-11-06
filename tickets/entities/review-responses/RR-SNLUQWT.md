---
id: RR-SNLUQWT
type: review-response
title: days_since and rrule_next ignore WithClock
finding: rela.days_since and the default after date of rela.rrule_next called time.Now directly, so a test pinning WithClock got a fixed rela.today but a live days_since and rrule_next.
severity: minor
resolution: Both helpers now close over the runtime's single clock reading; TestDateHelpersUseRuntimeClock pins it.
status: addressed
---
