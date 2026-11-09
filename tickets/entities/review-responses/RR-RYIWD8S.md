---
id: RR-RYIWD8S
type: review-response
title: ErrStale reported as already running
finding: A stale version from a concurrent tick was reported as ErrPeerBusy.
severity: nit
resolution: startManualRun reads again once on ErrStale and reports busy only on ErrRunActive. TestStartManualRun_RetriesAStaleVersion.
status: addressed
---
