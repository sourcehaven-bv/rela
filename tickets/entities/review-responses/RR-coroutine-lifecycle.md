---
id: RR-coroutine-lifecycle
type: review-response
title: Coroutine lifecycle and cleanup not fully specified
finding: |
  The Go code snippet shows `FlowRuntime` with a coroutine field but doesn't address:
  
  1. When is the coroutine created? (on first emit? on flow start?)
  2. How is the coroutine cleaned up on error/cancel?
  3. What happens if script calls emit() outside a flow context?
  4. What if script creates its own coroutines that also yield?
  5. How to distinguish flow yields from user coroutine yields?
  
  The gopher-lua pattern from research shows:
  ```go
  co, _ := L.NewThread()  // Creates coroutine
  st, err, values := L.Resume(co, fn)  // Resume with return values
  ```
  
  Recommendation: Document the full lifecycle:
  1. `rela flow script.lua` → Load script into new LState
  2. Wrap script in coroutine via `L.NewThread()`  
  3. First `Resume()` starts script execution
  4. `emit()` yields with form spec as value
  5. Go code receives yield, calls transport
  6. `Resume()` with event table
  7. Loop until `ResumeOK` (script done) or `ResumeError`
severity: minor
status: addressed
resolution: Added coroutine lifecycle diagram and implementation details explaining the full create/resume/cleanup cycle, emit() implementation, and nested coroutine handling.
---
