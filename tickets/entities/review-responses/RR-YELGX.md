---
id: RR-YELGX
type: review-response
title: get(key, opts?) signature has undocumented options
finding: The API surface lists `opts?` on `get` but none of the four options (`ttl`, `persist`, `bypass`, `refresh`) have defined behavior on a bare `get`. Either drop `opts` from `get` entirely or enumerate the honored options with explicit semantics.
severity: significant
resolution: 'Addressed in API surface section and AC 12: `get(key)` accepts no options; signature has no `opts?` parameter. `set` accepts only `ttl`. `memoize` accepts `ttl` and `bypass`. Unknown option keys raise a Lua error (prevents `refersh` typo silent ignore).'
status: addressed
---
