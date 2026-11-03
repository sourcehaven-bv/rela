---
id: RR-ZL35G2Q
type: review-response
title: A failed later refresh voids the load before it
finding: '[security] viewTicket ordered fetches by request, not by answer. The live subscription''s PUT starts a refresh after every switch; when that refresh failed, the load before it was skipped, so the old entity stayed under the new route, and a refused load was ignored so held content stayed on screen.'
severity: critical
resolution: Tickets now order by answers that landed (viewLanded). A refusal for the same target always drops held content. Pinned by the two 'a load and the live refresh' tests, which fail on the previous fix.
status: addressed
---
