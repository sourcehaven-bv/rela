---
id: RR-GL0TQAV
type: review-response
title: Redis Backend leaks connections when Start fails
finding: In the fork redis Backend now returns an error when mgr.Start or server.Start fails but leaves the client, inspector and server connection open.
severity: significant
reason: rela does not use the redis backend, so no rela process can hit these error paths. Moved to TKT-TD68KKT so the fix gets a reviewed fork PR instead of a second unreviewed merge to the sourcehaven branch.
status: deferred
---
