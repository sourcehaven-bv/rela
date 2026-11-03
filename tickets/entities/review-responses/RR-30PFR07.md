---
id: RR-30PFR07
type: review-response
title: File feed read errors were silent
finding: Stat, Open and ReadAt errors in the tail returned without a log at every poll. A journal owned by another user (mode 0600) would never be read and nobody would know.
severity: minor
resolution: The tail logs read errors at most once a minute. docs/realtime.md says the processes sharing a project must run as the same user.
status: addressed
---
