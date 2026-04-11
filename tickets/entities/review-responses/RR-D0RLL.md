---
id: RR-D0RLL
type: review-response
title: context.Canceled classified as timeout
finding: context.Canceled is classified as timeout kind, but cancellation is different from timeout. Scripts retrying on timeout would incorrectly retry canceled requests.
severity: critical
resolution: Added separate 'canceled' error kind for context.Canceled
status: addressed
---
