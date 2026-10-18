---
id: RR-GS856I
type: review-response
title: SQL DropTypeRecord count has no usable index and is not serialized with creates
finding: sqlchannels/retention.go:238 runs COUNT(*) WHERE type=? while channels_type_state_idx leads with tomb, so each undeclared type costs a sequential scan every hour. On pg, channel creation takes the anchor lock, not typeRecordsLock, so the "one step" claim in store.go does not hold across processes.
severity: minor
resolution: Added channels_type_state_idx, and both channel create and DropTypeRecord take the per-type typeLock.
status: addressed
---
