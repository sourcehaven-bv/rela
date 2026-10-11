---
id: TKT-6QC5EL
type: ticket
title: 'Realtime 2/1a: lease storage (kv with file lock, postgres, conformance)'
kind: enhancement
priority: high
effort: m
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

## Description

Phase 2 of FEAT-GWF8YH, slice 1a. Storage for advisory leases, modelled on
internal/piles: internal/lease value types, errors and Store interface;
lease/kvlease (one JSON document in the cache-dir state.KV under a mutex plus a
cross-process file lock, moved from kvpiles to internal/filelock); lease/pglease
(migration 0024); lease/leasetest conformance. Nothing is wired in, so behaviour
does not change.

Plan: .ignored/phase2-plan.md, section "Ticket 1 REVISED" (Model, AC1, AC2).
