---
id: RR-ZCL8D
type: review-response
title: refresh on bare set is meaningless - API is inconsistent
finding: '`set` already unconditionally writes; `refresh: true` is a no-op there. `bypass: true` on `set` is also meaningless. Both options exist only for `memoize`. Either scope the options table per-function or make the plan explicit about silent ignore.'
severity: significant
resolution: 'Addressed in AC 12: options are scoped per-function. `set` accepts only `ttl`; `memoize` accepts `ttl` and `bypass`. Unknown options on any function raise a Lua error. The `refresh` option has been collapsed into `bypass` (the two were speculatively split; a single option suffices for v1).'
status: addressed
---
