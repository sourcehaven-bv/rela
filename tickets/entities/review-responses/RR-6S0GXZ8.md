---
id: RR-6S0GXZ8
type: review-response
title: '[security] No memory or rate bounds on rooms'
finding: '[security] MessageRateLimit, MaxConnections and the awareness caps are unset and nothing caps a room''s document size. One writable connection can stream 2 MiB updates or large awareness states without limit and exhaust the single server process.'
severity: significant
resolution: 'Added limits: 4096 connections, 16 per principal (HTTP 429), 32 MiB room updates (close 4413), message rate 50/s burst 200, awareness caps. Tests: TestCollab_ConnectionsPerPrincipalAreCapped and TestCollab_RoomSizeIsCapped.'
status: addressed
---
