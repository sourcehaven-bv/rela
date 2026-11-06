---
id: RR-51RD0SJ
type: review-response
title: A body line can forge a conflicted file
finding: fsstore writes the body verbatim and a column-0 <<<<<<< line makes the file read as conflicted. A writer can make a row unreadable.
severity: minor
reason: Pre-existing and independent of this change. After the SkipUnreadable narrowing the only new effect is that the row is hidden from role_relations readers which is the narrowing direction. Tracked as BUG-GYM7ZBS.
status: deferred
---
