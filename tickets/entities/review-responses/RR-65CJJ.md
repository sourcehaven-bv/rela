---
id: RR-65CJJ
type: review-response
title: SetNow documentation should lock invariant
finding: Cache.SetNow is safe because every current read path holds c.mu. If a future caller ever reads c.now outside the lock, tests with a closure-based fake clock will race. Document on SetNow that the returned time source must be safe to call under c.mu and is exclusively invoked from locked methods.
severity: minor
resolution: 'Expanded SetNow doc comment with an INVARIANT section: the replacement function must be safe to call under c.mu and is exclusively invoked from locked methods. Future lock-free callers must snapshot under the lock first.'
status: addressed
---
