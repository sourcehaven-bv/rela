---
id: RR-0IAFXJ
type: review-response
title: Expiry timer re-arms after Stop
finding: A Post after Services.Close arms a new AfterFunc that nothing stops.
severity: minor
resolution: A stopped flag set by Stop prevents scheduleExpiry from re-arming.
status: addressed
---
