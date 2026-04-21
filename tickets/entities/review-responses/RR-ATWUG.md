---
id: RR-ATWUG
type: review-response
title: Coroutine interaction with memoize untested
finding: 'If `fn` inside memoize yields (e.g. calls `rela.flow.emit`), the coroutine suspends mid-call. The mutex is released (per AC 13), but the cacheEntry being built is incomplete. Worth one explicit test: `memoize` wrapping a function that calls `rela.flow.emit`.'
severity: significant
resolution: v1 does not test the coroutine+memoize combination. The coroutine path already works correctly by construction because memoize releases the mutex across `fn` (AC 13) and `set` is only called after `fn` returns normally. Interrupted flows leave the cache untouched, which is the correct behavior. Adding a test would require scaffolding a flow scenario that doesn't exist elsewhere in the test suite; deferred until a real bug or use case lands.
reason: 'The flow/coroutine path is out of scope for v1 verification. The mutex-release semantic (AC 13) is correct regardless of coroutines: on yield, the lock is not held; on resume, `fn` continues normally; if the user interrupts, `set` is never called and cache stays consistent. If a real flow-script-using-memoize bug appears post-ship, we''ll add the test then.'
status: deferred
---
