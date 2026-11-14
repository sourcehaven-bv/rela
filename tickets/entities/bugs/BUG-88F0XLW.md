---
id: BUG-88F0XLW
type: bug
title: Lua heap use is unbounded per tenant in a shared process
description: 'gopher-lua has no allocator hook. In tenant mode every tenant''s scripts share one Go heap. The bounds in internal/lua/bounds.go cap the registry, call stack, string.rep, string.format and table.concat, but string concatenation (..), gsub and table growth are bounded only by the script timeout and the process memory limit. One tenant can exhaust the heap for all tenants. A sampled heap watchdog was rejected because the heap is shared and it would cancel the wrong script. Options: per-state allocation accounting in a gopher-lua fork, or running tenant scripts out of process with their own memory limit.'
priority: medium
effort: m
status: backlog
---
