---
id: RR-0I1BQA2
type: review-response
title: A hung key list reload keeps serving a revoked key
finding: The reloading request holds the mutex across the file read; a stuck read made every other request serve the stale keyring forever.
severity: minor
resolution: Source.Verify refuses every key with ErrStale once the keyring is older than 3 intervals and the reload lock is held. TestSource_HungReloadFailsClosed pins it; docs state the 30 s bound.
status: addressed
---
