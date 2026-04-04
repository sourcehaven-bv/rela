---
id: RR-pcall-yield
type: review-response
title: pcall + coroutine.yield incompatibility in gopher-lua
finding: |
  The error handling design recommends using `pcall(rela.flow.emit, form)` for graceful error handling. However, gopher-lua has a known issue (#306) where `coroutine.yield()` inside a `pcall()` throws "can not yield from outside of a coroutine" error, unlike standard Lua 5.1.
  
  Since `rela.flow.emit()` internally yields, wrapping it in `pcall()` will fail. The plan's error handling example is broken:
  ```lua
  -- This will NOT work in gopher-lua!
  local ok, result = pcall(rela.flow.emit, form)
  ```
  
  Options:
  1. Remove pcall recommendation from docs (let errors propagate)
  2. Implement custom error handling that doesn't use pcall
  3. Return nil, err instead of raising (breaks consistency with other rela bindings)
severity: significant
status: addressed
resolution: Removed pcall example from documentation. Error handling section now explains that pcall cannot wrap emit() and why this is acceptable (validation errors are script bugs, transport errors are unrecoverable, user cancellation uses cancel action).
---
