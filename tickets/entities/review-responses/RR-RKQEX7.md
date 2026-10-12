---
id: RR-RKQEX7
type: review-response
title: filechannels Head reads and decodes the whole last segment under the lock
finding: 'Head takes the semaphore and flock then folds the full last segment (up to ~16 MB). T3 polls Head, so polling blocks writers. Fix: parse only the last complete line of the newest non-empty segment.'
severity: significant
resolution: Head and Append read the newest segment from its end, skipping patches and a torn tail.
status: addressed
---
