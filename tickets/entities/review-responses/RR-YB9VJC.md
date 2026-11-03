---
id: RR-YB9VJC
type: review-response
title: Replay into per-subscriber buffers is costly and complex
finding: Pre-filling a channel with up to 4096 events costs about 600 KB per reconnect, holds the bus lock, and needs lag detection and pump-side close.
severity: significant
resolution: 'Per-type state: Subscribe and Take are O(types); no buffers, no lag, no pump-side close.'
status: addressed
---
