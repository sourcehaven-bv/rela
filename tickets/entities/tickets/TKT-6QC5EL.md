---
id: TKT-6QC5EL
type: ticket
title: 'Realtime 2/1: advisory lease store with claim, renew, release'
kind: enhancement
priority: high
effort: l
status: backlog
---

## Description

Phase 2 of FEAT-GWF8YH, first slice. Agents and people claim an entity for a
time-limited, advisory lease with a fencing token. A store seam in the
schedulerstate pattern (kv with a cross-process lock, postgres, conformance
suite), a lease service, MCP tools
claim_entity/renew_lease/release_lease/list_leases, HTTP routes, _lease on
entity and list responses, a reaper, and lease changes as a wake source for live
views and watch.

Out of scope: enforcing tokens on writes, admin force-break, SPA, presence.

Plan: .ignored/phase2-plan.md (Ticket 1).
