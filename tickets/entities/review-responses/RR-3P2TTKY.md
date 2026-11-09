---
id: RR-3P2TTKY
type: review-response
title: 'Design: skipping StartWatching and the scheduler turns off live updates and realtime workers'
finding: StartWatching starts the store event bridge (live updates, AC1); startScheduler also starts WatchLeases, RunRealtimeWorker and FollowChannels.
severity: significant
resolution: Each tenant resident starts StartWatching plus the lease watcher, realtime worker and channel follower and stops them on Close. The scheduler stays off (TKT-M3SHYE0).
status: addressed
---
