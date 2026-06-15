---
id: RR-8CADXI
type: review-response
title: 'Lossy select-default broker drop makes delivered-set eviction probabilistic: a dropped became-unreadable update means no eviction → later delete leaks'
finding: 'broadcastEvent drops to slow clients via select-default, buffer 4 (watcher.go:43,65). The delivered-set''s ONE handled sub-case (direct-update visibility flip) requires the handler to SEE the evicting update — but under a write burst (attacker-inducible) that very update can be the dropped event, leaving stale true → delete leaks. Even the sub-case the design claims to handle is unreliable. Type-fallback (gate delete by live ReadQuery type-verdict) is immune: it re-asks the live gate at delete time, never relies on having observed an intermediate event.'
severity: significant
resolution: Moot under the cacheId design — no server-side delivered-set to keep consistent, so lossy broker drops can't cause an eviction-miss leak. A dropped create just means the client never learns the cacheId→entity mapping, so a later opaque delete for it is correctly ignored (client holds nothing). No stale-allow path exists when there's no per-connection trust state.
status: addressed
---
