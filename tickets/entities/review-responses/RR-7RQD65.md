---
id: RR-7RQD65
type: review-response
title: Untrusted text in new output
finding: Warning details and transition names come from other people's data and config; printed raw they can carry terminal escapes.
severity: minor
resolution: Plan and code print every part of new output through printable, as the other channel output does.
status: addressed
---
