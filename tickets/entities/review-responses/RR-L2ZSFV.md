---
id: RR-L2ZSFV
type: review-response
title: Git sync returns go-git error text with a 200
finding: handlers_git.go puts err.Error() from analyze and sync in resp.Error.
severity: minor
reason: Not a 500 path; tracked in BUG-E3V44J with the other non-500 leaks.
status: deferred
---
