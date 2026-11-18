---
id: BUG-ZO33J9M
type: bug
title: 'tenant mode: webhook replay dedup resets when a tenant is evicted'
description: Each tenant App keeps its own seen-set of IdP webhook ids. Evicting the tenant drops the set, so a captured webhook can be replayed once within its JWT lifetime after the tenant reopens.
priority: medium
why1: The webhook seen-set was an in-memory field on the tenant App, and evicting the tenant dropped it.
why2: The seen-set was written when the App lived as long as the process; tenant mode (TKT-D6MF3SW) made the App evictable.
why3: Per-process state on the App was not reviewed for eviction when the tenant registry was added.
why4: The tenant-mode design listed what eviction stops, but did not list per-App security state such as replay protection.
why5: Security state was kept in process memory by default; there was no rule that state which must outlive a tenant App goes in state.KV.
prevention: The seen-set lives in the tenant state.KV with a TTL and fails closed (503 with Retry-After) when state is unavailable. Pinned by TestSeenSet_SurvivesEviction and TestSeenSet_FailsClosed. Other per-tenant lifecycle state was reviewed in TKT-N3X235N.
started: "2026-11-16"
completed: "2026-11-16"
status: done
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
