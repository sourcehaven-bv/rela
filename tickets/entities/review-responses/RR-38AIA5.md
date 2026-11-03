---
id: RR-38AIA5
type: review-response
title: A 304 can serve a stale roll-up
finding: The ETag excludes roll-ups, but the GET still answered 304 on a matching If-None-Match.
severity: minor
resolution: A type with roll-ups never answers 304, as with leases. Documented in the guide.
status: addressed
---
