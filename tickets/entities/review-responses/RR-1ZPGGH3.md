---
id: RR-1ZPGGH3
type: review-response
title: filechannels empties the trash outside the lock
finding: A removal emptied ~trash after unlock, which could race a Delete or Purge that moves a channel into the trash.
severity: significant
resolution: mutateRemove empties the trash in its under-lock after hook. Tested in TestRemoveEmptiesTrashUnderLock.
status: addressed
---
