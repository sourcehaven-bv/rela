---
id: RR-KD05YX5
type: review-response
title: '[security] Hint points to RELA_DATAENTRY_USER without a non-loopback warning'
finding: '[security] The new advice names $RELA_DATAENTRY_USER; an operator on a network bind who follows it silences the warning and makes every caller that user, with no startup warning.'
severity: minor
resolution: main.go warns when the env var is set on a non-loopback bind, next to the --principal-header warning; GUIDE-server-security says so.
status: addressed
---
