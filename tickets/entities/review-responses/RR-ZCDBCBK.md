---
id: RR-ZCDBCBK
type: review-response
title: Session display check reads HasSession through BusyFacts
finding: checkChannelBlock calls the busy-column method only to learn whether a type has a session.
severity: minor
resolution: The BusyFacts godoc now names both uses. A rename touches the realtime schema adapter and its tests, which is out of proportion for this ticket.
status: addressed
---
