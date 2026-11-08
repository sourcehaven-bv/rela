---
id: BUG-ZO33J9M
type: bug
title: 'tenant mode: webhook replay dedup resets when a tenant is evicted'
description: Each tenant App keeps its own seen-set of IdP webhook ids. Evicting the tenant drops the set, so a captured webhook can be replayed once within its JWT lifetime after the tenant reopens.
priority: medium
status: backlog
---

## Symptom

The webhook dedup set (`internal/dataentry/webhook.go`) lives on the tenant App.
Tenant mode (TKT-D6MF3SW) evicts idle tenants, which drops the set. A captured,
still-valid webhook JWT can then be accepted a second time.

## Impact

Limited: the handler refetches authoritative data from the IdP, and the JWT
expiry bounds the window. Tenant mode is not released yet.

## Fix direction

Keep the seen-set in the tenant's `state.KV` so it survives eviction. Tracked
with the other lifecycle work in TKT-N3X235N (review finding RR-NAQHCBE).
