---
id: RR-5YKD42X
type: review-response
title: Resident cap does not bound open pools
finding: A tenant evicted while an SSE stream holds it stays open, so open pools follow tenants with an open tab rather than max_resident.
severity: significant
resolution: 'Added a hard ceiling (tenant.WithMaxOpen / max_open, default 2x max_resident) counting resident, pinned, opening and closing tenants. At the ceiling an idle resident is evicted and waited for; otherwise Acquire returns ErrAtCapacity, which the host answers 503 with Retry-After. Reconnect hints to pinned streams are in TKT-N3X235N. Test: TestRegistry_RefusesAboveCeiling.'
status: addressed
---
