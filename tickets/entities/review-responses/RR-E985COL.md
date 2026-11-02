---
id: RR-E985COL
type: review-response
title: Feed belongs to one assembly but Local outlives it
finding: Local is shared across assemblies but the channel store is rebuilt per assembly; overlapping generations would start two followers.
severity: significant
resolution: 'A hintRelay in shared state next to realtimeLocal; each assembly attaches its endpoints by atomic swap. FollowHints is single-flight per Local. Test: a schema reload keeps one follower.'
status: addressed
---
