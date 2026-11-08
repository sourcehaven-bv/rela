---
id: RR-NAQHCBE
type: review-response
title: Webhook replay dedup resets on eviction
finding: The seen-set lives on each tenant App, so eviction forgets it and a captured webhook can be replayed within its JWT lifetime.
severity: minor
reason: 'Impact limited: the action refetches authoritative data and JWT expiry bounds the window. Moving the set to the tenant''s state.KV is tracked in TKT-N3X235N.'
status: deferred
---
