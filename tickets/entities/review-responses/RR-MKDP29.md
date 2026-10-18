---
id: RR-MKDP29
type: review-response
title: Rename batch is filed confirmed, so a tiny detached_retention can purge before undo
finding: channelguard.go:211 files the BeforeRename batch with pending=false, and validate.go:161-165 accepts detached_retention as low as 1s. A worker woken by any delete can purge it before a failed rename's Restore. File it pending, or set a floor of at least OrphanGrace.
severity: minor
resolution: The rename batch is filed pending and confirmed only in EntityRenamed, and detached_retention must be at least 1h.
status: addressed
---
