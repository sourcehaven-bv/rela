---
id: RR-REQI95
type: review-response
title: StartWatching cannot restart after StopWatching
finding: The Bus closes permanently; a restart would log ErrClosed and leave live updates dead.
severity: minor
resolution: StopWatching doc now states an App cannot start watching again; rela-server builds a new App per configuration, so no caller restarts.
status: addressed
---
