---
id: RR-MG8GD1
type: review-response
title: Line citation for can_post
finding: read_channel can_post comes from Read calling c.postable() at internal/realtime/read.go:127 and messageTypes setting CanPost at read.go:222, not read.go:216.
severity: nit
resolution: The citation is corrected to `Read` calling `postable()` at `read.go:127` and `messageTypes` setting `CanPost` at `read.go:222`.
status: addressed
---
