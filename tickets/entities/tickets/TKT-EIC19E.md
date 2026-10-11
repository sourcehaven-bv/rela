---
id: TKT-EIC19E
type: ticket
title: 'Realtime 2/1b: lease service with access checks, rename/delete hooks, wake'
kind: enhancement
priority: high
effort: m
started: "2026-10-11"
completed: "2026-10-12"
status: done
---

## Description

Phase 2 of FEAT-GWF8YH, slice 1b. lease.Service over the 1a store:
claim/renew/release with read and update checks against the WriteTarget face,
renew re-checking access, caps and input validation, EntityRenamed/EntityDeleted
hooks through the alias fanout, Subscribe wake channels, and Watch (version poll
plus expiry timer) for long-running processes; appbuild wiring.

Plan: .ignored/phase2-plan.md, section "Ticket 1 REVISED" (Service, AC3, AC4).
