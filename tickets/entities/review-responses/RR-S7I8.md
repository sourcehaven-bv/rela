---
id: RR-S7I8
type: review-response
title: 'Picker race condition: modal close vs. in-flight search response'
finding: Plan reuses CommandPaletteModal's debounce-and-abort pattern. CommandPaletteModal calls `cancelInflight()` inside the `open` watcher when transitioning to closed (line 126). The plan implies the new picker mirrors that pattern but doesn't say so explicitly. Without it, a network response can arrive AFTER `select` was emitted and the parent has reused the picker for a different cursor position — the stale result list flashes into a closed-modal DOM. The plan should reference the abort-on-close requirement explicitly, with a test.
severity: minor
resolution: 'Plan §Approach §1: explicit ''Abort on close'' bullet referencing CommandPaletteModal line 126. New picker test row in the test plan asserts AbortController.abort() fires on open->false transition.'
status: addressed
---
