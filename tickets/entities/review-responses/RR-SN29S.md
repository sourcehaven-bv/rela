---
id: RR-SN29S
type: review-response
title: SafeFS observer read-release-call pattern is not obviously correct
finding: safefs.go:108-114 reads observer under RLock, releases, then calls. If another goroutine calls OnPostWrite(nil) between release and call, the just-released observer still fires. Harmless today (observer is FSStore.RecordWrite on long-lived object), but pattern looks race-prone at first glance and will confuse static analysis.
severity: nit
reason: 'Pattern is safe today: observer is FSStore.RecordWrite which closes over a sync-safe LRU. Changing to call-under-lock risks deadlock if a future observer blocks or re-enters WriteFile. The current release-before-call semantics are documented with comments; that''s the right tradeoff. Re-visit only if an observer with stateful teardown is introduced.'
status: wont-fix
---
