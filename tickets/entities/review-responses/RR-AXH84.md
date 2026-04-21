---
id: RR-AXH84
type: review-response
title: absPath panics on resolve failure; ValidateID/resolve mismatch is a real scenario
finding: absPath panics on resolve failure. ValidateID accepts IDs like 'CON' (Windows reserved), which resolve rejects. A user creating such an entity would crash the process instead of getting a clean error.
severity: significant
resolution: 'Changed absPath to return empty string on resolve failure (not panic). Forget/isEntityPath/isRelationPath/StartWatching all updated to no-op on empty result: a key that can''t resolve was never written, so the LRU doesn''t have it and the watcher can safely skip it. Clean no-op failure mode matches the rest of the codebase.'
status: addressed
---
