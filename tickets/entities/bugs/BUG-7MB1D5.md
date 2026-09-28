---
id: BUG-7MB1D5
type: bug
title: Gantt, feeds, CalDAV, webhooks, command context and sync are empty for faced types
description: These surfaces query default rows only; a faced source type returns nothing with no error or config warning.
priority: medium
effort: m
status: backlog
---

## Problem

Reported by the face-awareness inventory, not yet verified. These surfaces use
an `EntityQuery{}` with neither all-states nor a world, so a faced type returns
nothing, with no error:

- gantt (`gantt_handler.go:345,568`)
- iCal feeds (`feed_handler.go:154`)
- CalDAV (`caldav_write.go:505,736`)
- custom webhooks (`webhook_routes.go:409,546`)
- the command list context (`helpers.go:633`)
- sync (`pgstore/manifest.go:28-55`, `dataentry/sync.go:168`)

Nothing at config load warns that a faced type is the source of a surface that
cannot see faces.

## Expected

Each surface resolves faced types through a world, or config load rejects a
faced source type it cannot serve.
