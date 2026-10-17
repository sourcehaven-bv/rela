---
id: RR-YSEL86
type: review-response
title: Every delete pays for the guard
finding: Each delete does a second relation collection, owned preparation and ACL evaluation plus channel lists, even with no channels; the conflict checks also refuse deletes where the guard did nothing.
severity: minor
resolution: The guard returns a scope only when the root or its owned children have channels; otherwise the pre-pass is skipped, and the conflict checks run only when something was detached.
status: addressed
---
