---
id: RR-UKZV3Z
type: review-response
title: Synchronous syslog write couples latency to journald
finding: syslog.Writer writes under a mutex to a blocking socket before the response is finished.
severity: minor
resolution: 'Documented under Operational limits. An async drop-on-full queue is not added: one small datagram per request to a local socket; revisit if it shows up.'
status: addressed
---
