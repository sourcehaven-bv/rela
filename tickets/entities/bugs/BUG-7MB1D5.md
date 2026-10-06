---
id: BUG-7MB1D5
type: bug
title: Gantt, feeds, CalDAV, webhooks and command context are empty for faced types
description: These surfaces query default rows only; a faced source type returns nothing with no error or config warning.
priority: medium
effort: m
why1: CalDAV writes and webhook finds addressed a bare id or the default rows; a faced type has no implicit row so they missed it.
why2: These surfaces predate faces and were never given a world or a write target.
why3: Faces were opt-in per surface, so each surface had to be found and fixed by hand; the inventory found them after the fact.
why4: There was no single resolver for bare-id writes until WriteTarget.
why5: The read and write seams accepted an address without a face, so face-blindness compiled and ran silently.
prevention: Every route now binds the default world (gantt, feeds and command context read through it); CalDAV and webhook writes resolve through WriteTarget; webhook find.face is validated at load. Pinned by TestCalDAV_WriteAddress, TestWebhookRoutes_FindFace and TestValidateWebhooks_FindFace.
status: done
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
- ~~sync~~ (removed with the sync feature, TKT-7IZHP0 PR 10)

Nothing at config load warns that a faced type is the source of a surface that
cannot see faces.

## Expected

Each surface resolves faced types through a world, or config load rejects a
faced source type it cannot serve.
