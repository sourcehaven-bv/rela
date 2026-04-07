---
id: RR-MJMQ
type: review-response
title: renderListItemTable is free function instead of method
finding: Inconsistent with the rest of the file where renderers are (r *Runtime) methods. If access to r.L is ever needed, refactor required.
severity: nit
resolution: renderListItemTable refactored to a method on *Runtime, consistent with the rest of the renderer methods in the file.
status: addressed
---
