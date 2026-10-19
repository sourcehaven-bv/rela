---
id: RR-BRN38A
type: review-response
title: Note's rule-type cache ignores a Degraded flip on the same Config pointer
finding: 'internal/realtime/trigger.go:153-164. The cache is keyed on snap.Config only. A degraded snapshot keeps the earlier Config pointer, so a cache built while healthy keeps accepting Notes while degraded (RunRound then discards them). It is harmless now but brittle. Fix: key the cache on (Config, Degraded).'
severity: nit
resolution: The Note cache key now includes both the Config pointer and the Degraded flag.
status: addressed
---
