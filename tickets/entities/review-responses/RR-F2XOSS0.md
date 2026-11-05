---
id: RR-F2XOSS0
type: review-response
title: No server keepalive for half-open peers
finding: 'ygo sends no pings and clears the read deadline after the first message, so a half-open TCP peer stays in its room until the connection lifetime cap. Accepted: the 10 minute cap bounds it and clients drop stale awareness after 30s.'
severity: minor
reason: Bounded by the 10 minute connection lifetime cap; clients drop stale awareness after 30s.
status: wont-fix
---
