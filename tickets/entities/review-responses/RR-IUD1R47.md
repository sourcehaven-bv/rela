---
id: RR-IUD1R47
type: review-response
title: Wrong comment on EntityRenamed
finding: local.go said every process gets its own rename hook; the hook fires only in the process that renamed.
severity: minor
resolution: 'Comment corrected: the drops stay local, the alias hooks send the moved channels, and other processes keep their own entries under the old id until they expire.'
status: addressed
---
