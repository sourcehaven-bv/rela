---
id: RR-KR1JVE
type: review-response
title: postgres build falls back to shared cache dir for channels
finding: 'When pgstore.HandleFor is nil buildChannels uses filechannels in paths.CacheDir, shared across tenants. Piles and leases refuse this. Fix: in-memory backend or fail.'
severity: significant
resolution: Postgres build without pgstore uses an in-memory channel store, matching piles and leases.
status: addressed
---
