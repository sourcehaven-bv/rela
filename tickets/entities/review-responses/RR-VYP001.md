---
id: RR-VYP001
type: review-response
title: 'Rebuild released the index lock before deleting the directory'
finding: 'New closed an outdated index and then removed its directory. A second process could open it in between, and one process would delete the other live index. The format check made this happen on every upgrade.'
severity: significant
resolution: 'An outdated index is now emptied in place while this process holds the lock, then stamped. The directory is only removed for a corrupted index, as before.'
reason: 'Emptying in place keeps the lock for the whole rebuild.'
status: addressed
---
