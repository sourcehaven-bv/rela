---
id: TKT-V270TMQ
type: ticket
title: 'Property test: two clients autosaving one entity'
kind: test
priority: high
effort: m
status: backlog
---

## Description

Drive two useAutoSave instances against the fake API (per-field versions, 412)
under fc.scheduler(). Property: every field ends with one client's write, and
any lost write is reported as a conflict. The merge rules in frontend/CLAUDE.md
are the checks. Include a deliberate fault the test must catch.

Part of the SPA property-testing plan. Start after TKT-7LTLHW3 merges; it
provides the fake API, the contract test and the scheduler setup.
