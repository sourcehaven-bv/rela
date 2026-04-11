---
id: BUG-DW6H
type: bug
title: Data-entry SPA refetches and re-renders open lists on every SSE refresh event, making the UI unusable when files change frequently
description: 'When the file watcher fires (entity edit, git operation, IDE save, automation, etc.) the broker broadcasts a refresh SSE event. The SPA reacts by refetching the current list and re-rendering, which detaches and recreates every row DOM element. If file activity is high (multiple edits per second), the user cannot click anything in a list — every click target is destroyed before the click handler fires. Reproduced by frontend/stress watcher-pressure scenario: 24/83 (29%) entity-open clicks fail with ''element was detached from the DOM, retrying'' under 4 concurrent users + ~75 file-touch events in 60s. Server-side metrics are clean (schema p99 < 5ms, no 5xx, no goroutine growth) — this is purely a frontend SSE handling / re-render problem.'
priority: medium
effort: s
why1: Clicks on list rows fail because the row DOM element is destroyed mid-click.
why2: The list view rebuilds its entire row tree every time it receives an SSE refresh event.
status: ready
---
