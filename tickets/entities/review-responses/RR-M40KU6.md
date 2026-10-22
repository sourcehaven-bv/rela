---
id: RR-M40KU6
type: review-response
title: CheckOwningEdgeLeaving exported for the fallback only
finding: CheckOwningEdgeLeaving was exported for writeBoundedFallback and has no callers outside entitymanager now.
severity: minor
resolution: Unexported as checkOwningEdgeLeaving.
status: addressed
---
