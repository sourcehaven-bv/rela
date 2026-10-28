---
id: RR-GOKV3S
type: review-response
title: Hashing every fs attachment at open slows every CLI command
finding: loadAttachmentsIndex runs on every store open and the index is not refreshed when files change on disk.
severity: significant
resolution: 'Plan: hash on write from the stream; on list compute lazily and cache in attachMeta keyed on size and mtime; re-stat on list.'
status: addressed
---
