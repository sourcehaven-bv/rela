---
id: RR-B9OZQZ
type: review-response
title: Verify runs before checkpoint and rename
finding: Uncheckpointed WAL content would be lost by renaming only the main file while verify had already passed.
severity: significant
resolution: 'Plan updated: Checkpoint(TRUNCATE), close, assert no -wal/-shm, rename, then reopen the final rela.db read-only and verify that.'
status: addressed
---
