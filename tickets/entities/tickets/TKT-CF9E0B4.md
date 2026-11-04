---
id: TKT-CF9E0B4
type: ticket
title: Calendar view events from the calfeed model
kind: refactor
priority: medium
effort: m
status: backlog
---

## Description

The SPA maps entities to calendar events itself (`useCalendarEvents.ts`), while
feeds and CalDAV use the `calfeed` event model. Have the calendar endpoint
return events built with `calfeed`, so all three agree, and remove the client
mapping.

Part of FEAT-QRI3CFR (step 1.4).
