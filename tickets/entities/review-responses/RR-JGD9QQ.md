---
id: RR-JGD9QQ
type: review-response
title: Demo postgres used trust auth on TCP
finding: Any local user could connect as superuser, including after SEQTRACE_KEEP_DB=1.
severity: minor
resolution: initdb uses scram-sha-256 with a random per-run password; the docker path uses the same password.
status: addressed
---
