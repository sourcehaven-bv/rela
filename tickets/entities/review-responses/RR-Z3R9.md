---
id: RR-Z3R9
type: review-response
title: Action handler missing workspace write lock
finding: Action scripts mutate workspace but plan doesn't take a.mu.Lock(). All existing write handlers take the lock. Concurrent clicks will race and corrupt graph/cache.
severity: critical
resolution: Handler takes a.mu.Lock() for the duration of script execution, matching the clone handler pattern.
status: addressed
---
