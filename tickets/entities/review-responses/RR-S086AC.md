---
id: RR-S086AC
type: review-response
title: Failed probe disables dragging permanently
finding: A network or 5xx error cached a false verdict.
severity: minor
resolution: Only 403/404 cache false; other failures are asked again on the next hover. Test added.
status: addressed
---
