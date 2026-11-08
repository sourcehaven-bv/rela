---
id: RR-NJ7IG1V
type: review-response
title: 'Design: per-tenant teardown undefined; retire on eviction can thrash'
finding: App has no Close; the resident needs one ordered Close (Retire, workers, svc.Close, audit). Retiring streaming tenants on eviction forces reconnects that evict another tenant.
severity: significant
resolution: 'Resident Close order: Retire app, stop workers, svc.Close (owned audit sink last), relay shutdown. No Retire on eviction (avoids thrash); max_resident in tenants.yaml.'
status: addressed
---
