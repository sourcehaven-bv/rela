---
id: RR-VT3LUC
type: review-response
title: Lock godoc still describes a lock taken once per run
finding: 'internal/appbuild/channels_postgres.go storeLeaderLockFor (''a keyed advisory lock tried once per run ... none waits on it'') and the TryKeyedLock godoc (''the caller takes it once per run and skips the run when it is held ... a process that stops holding it (a crash, a lost connection) lets the next run of another process take over'') no longer match the worker, which holds it for its lifetime and cannot see a lost connection. Fix: update both comments to say the lock is held for the leader''s lifetime and what happens when the session drops.'
severity: nit
resolution: The storeLeaderLockFor and TryKeyedLock godocs now describe a lock held for the leader's lifetime, checked with HeldLock.Check, and what happens when the session drops.
status: addressed
---
