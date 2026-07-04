---
id: FEAT-OT4361
type: feature
title: Format-agnostic calendar/feed export
summary: rela emits time-bearing graph data as standard feeds (iCalendar, JSON; later Atom/RSS, CalDAV) built by a Lua-authored abstract event model and served over HTTP with deep links back into the data-entry app.
description: 'A read-only feed-export capability. User Lua scripts curate which entities become calendar events (e.g. tasks with a due date, classified overdue/due-today) and describe them as an abstract, format-agnostic event model. rela renders that model to a closed set of output formats (Phase 1: iCalendar VEVENT+VALARM and JSON; designed to later back Atom/RSS and a Phase-2 CalDAV server) and serves them over HTTP at a stable, per-feed-token-authenticated URL so calendar apps subscribe and alert natively. Events carry deep links to the entity in the data-entry SPA. rela stays the single source of truth; the feed is a read-only view. See RES-AHY3VS for the design.'
priority: medium
status: proposed
---
