---
id: RR-TMI6E5
type: review-response
title: Spec lists browser-only operations restish cannot call
finding: The spec listed /api/events and /api/git/{status,sync}. They are outside the non-browser CSRF exemption so every restish call gets 403 origin_missing.
severity: significant
resolution: Removed the three paths from the spec (exempting a git write would widen CSRF). TestOpenAPI_EveryOperationReachesAHandler now runs with the same-origin layer on and fails on 403.
status: addressed
---
