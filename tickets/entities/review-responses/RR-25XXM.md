---
id: RR-25XXM
type: review-response
title: 'GetCached must be bypassed (read side) for script: docs'
finding: 'Plan says ''Disk cache: off for script:'' — but only addresses the write path. handleV1Documents calls GetCached() before Render(); on a system that migrates a command: doc to script:, the old disk cache files are still there and GetCached will serve stale command:-rendered HTML. ContentHash doesn''t disambiguate renderer type.'
severity: significant
resolution: GetCached() is only called when cfg.Script == ''. Script renders bypass disk cache on both read and write. AC10 covers the stale-cache-not-served case.
status: addressed
---

From design-review on PLAN-78HJO.

Fix: either (a) Skip `GetCached` entirely when `cfg.Script != ""`. (b)
Incorporate renderer type and/or configID into the cache file name so old
command:-rendered files don't match new script: requests.

(a) is simpler, trivially correct, and matches "Lua caching is Lua's
responsibility via rela.cache.memoize."
