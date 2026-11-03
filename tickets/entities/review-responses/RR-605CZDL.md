---
id: RR-605CZDL
type: review-response
title: markResync repeats takeLocked's clearing
finding: markResync clears the subscriber's sets, which takeLocked also does when resync is set.
severity: nit
reason: Clearing at mark time frees the sets at once instead of holding up to 1024 keys per subscriber until the next take. The duplication is four lines in the same file.
status: wont-fix
---
