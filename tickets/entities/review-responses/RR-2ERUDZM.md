---
id: RR-2ERUDZM
type: review-response
title: Auto-read misses a notification that arrives while its entity is open
finding: The watcher in PileToggles.vue fired on the pile list length so a bumped notification for the open entity stayed unread.
severity: minor
resolution: The watcher follows whether the address is unread on an auto-read pile. New test 'reads a notification that arrives while its entity is open'.
status: addressed
---
