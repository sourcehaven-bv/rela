---
id: RR-NKTT2D
type: review-response
title: The acceptance criteria miss the riskiest behaviour
finding: '§6 has no AC for: useEntityChannels clearing on a refused list refetch (only useChannel has AC 8, and frontend/CLAUDE.md requires a denial test and a transient test per holding surface); descriptor reload after a realtime.yaml edit; ephemeral rendering and expiry; the edit flow; pending-promote release; 429 Retry-After; a channel deleted or archived while open; a message type removed from config (TypeMessageTypeGone 409). AC 11 also lacks the live revocation case (a realtime.yaml or acl change removes read, and the second context drops the panel without reload; the backend has TestLive_RealtimeReloadRevokes). Fix: add an AC for each, and name its test.'
severity: significant
resolution: Section 15 adds acceptance criteria with named tests for list clearing, descriptor reload, ephemeral expiry, edit, release, 429, lifecycle changes while open, a removed message type and live revocation (R16).
status: addressed
---
