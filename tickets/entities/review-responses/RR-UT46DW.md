---
id: RR-UT46DW
type: review-response
title: fs SetRead takes the project-wide lock and rewrites the whole reads.json on every mark
finding: 'The filechannels lock (semaphore plus file lock) is global to the store directory, not per channel. Each SPA mark (up to 1 per second per viewer) therefore serializes with every append, lifecycle change and Head in the project, across processes. It also rewrites and syncs a file that holds every reader of the channel. Fix: keep it, but note it in the fs scale note of docs/realtime.md. Make sure SetRead returns early, without taking the lock, when the seq is not above the marker read without the lock (monotonic, so a stale read only costs a redundant lock). Add a test that a no-op mark takes no lock.'
severity: minor
resolution: 'Kept, with an unlocked pre-check: a mark not above the marker returns without the lock. TestSetRead_NoOpTakesNoLock. Lock and rewrite noted in the fs scale note.'
status: addressed
---
