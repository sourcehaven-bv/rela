---
id: RR-68KJOJ2
type: review-response
title: Weak ETags from a proxy turn every update into a 412
finding: A gzip proxy may downgrade strong ETags to W/ and If-Match then never matches.
severity: minor
reason: Not seen with the rela server; a second 412 leaves the row for the next run and is reported as a conflict. Revisit if deployments behind such proxies report it.
status: deferred
---
