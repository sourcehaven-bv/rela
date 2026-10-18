---
id: RR-AGUSO5
type: review-response
title: Worker holds a blocking pg keyed lock without a deadline, starving attachment writes
finding: internal/appbuild/realtime_worker.go:186-188,306 takes lock.For(st).Acquire with no deadline and holds it for the worker's life. Non-leaders block holding a keyed slot and a pinned connection (pgstore/keyedlock.go:134 gives MaxConns/2 slots, shared with the AttachmentLocker). At pool_max_conns 2-3 attachment writes hang until their ctx expires. Use a try-lock with retry, like the version sweep.
severity: significant
resolution: The worker takes a non-blocking try-lock per run, uses no keyed slot and holds the connection only during the run.
status: addressed
---
