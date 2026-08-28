---
id: RR-1Y72FB
type: review-response
title: Enqueue released the lock before reaching the backend, so Close could complete underneath an accepted job
finding: Enqueue read the closed/started flags under q.mu.RLock, released the lock, then called into the backend under a different mutex (enqueueMu). Close takes q.mu.Lock across nq.Shutdown, so nothing prevented a job that had passed the closed check from being handed to a backend already shutting down — on the memory backend a send on a channel whose reader has gone, on postgres a query on a closing pool.
severity: significant
resolution: 'The read lock is now held across the backend enqueue, with the closed flag re-checked under it. Since Close holds the write lock across Shutdown, the two are mutually exclusive; readers still run concurrently, so this serializes enqueue against shutdown only, not against other enqueues. Lock ordering audited: only one site acquires both (enqueueMu then q.mu) and no path takes them in reverse, so no inversion is introduced.'
status: addressed
---
