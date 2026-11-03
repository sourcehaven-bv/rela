---
id: RR-HCLOG5
type: review-response
title: Partial Tombstone error discards the handle
finding: filechannels Tombstone returns (h, err) after moving the directory when the audit append or move fails (ops.go:507/530). BeforeDelete only checks err and returns a nil undo, so the delete is refused but channels stay detached; guardDeletes never undoes the failing batch.
severity: significant
resolution: channelGuard.tombstone untombstones a non-zero handle on a partial error before returning; TestChannelGuard_PartialTombstoneIsRestored covers delete and rename.
status: addressed
---
