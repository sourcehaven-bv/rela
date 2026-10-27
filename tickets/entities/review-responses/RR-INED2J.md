---
id: RR-INED2J
type: review-response
title: list_inbox makes the watch poll a fixed-rate amplifier
finding: 'The channel poll sets chanDirty unconditionally every 2 s (tools_watch.go), and Approach 9 matches every key for list_inbox. So each watch replays the whole inbox every 2 s even when nothing changed: P holder queries, ListWaiting, the anchor batch and the prime. On fs with any creator waiting role, that includes a walk of every channel meta.yaml. A replay longer than watchReplayTimeout (5 s) counts as incomplete and reruns at every 500 ms tick, which is a busy loop exactly when the inbox is slow. One watch may also hold 16 list_inbox targets that differ only in limit, and an identity may hold 4 watches. Fix: allow at most one list_inbox target per watch. Skip a poll replay when a cheap change token has not moved (for example a channel-store generation or max rev plus the store revision). Put the fs scale limit in docs next to the ListSessionsDue note, and add a test where a slow replay does not spin.'
severity: significant
resolution: 'Adopted in part: one list_inbox target per watch; poll replay at most every 15 s with backoff doubling after an incomplete replay; fs scale note in docs; slow-replay test. No change token: no backend has a channel generation and fs has none.'
status: addressed
---
