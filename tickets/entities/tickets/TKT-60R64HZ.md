---
id: TKT-60R64HZ
type: ticket
title: 'Property test and fix: DynamicForm across entity changes'
kind: test
priority: high
effort: m
status: backlog
---

## Description

DynamicForm is reused across entities, and loadEntity and the autosave
getEntityId have no guard against a late answer (found in TKT-7LTLHW3).
Property: the form shows and saves only the entity the route names. Write the
property first, then fix.

Part of the SPA property-testing plan. Start after TKT-7LTLHW3 merges; it
provides the fake API, the contract test and the scheduler setup.
