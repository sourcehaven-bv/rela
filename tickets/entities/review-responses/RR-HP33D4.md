---
id: RR-HP33D4
type: review-response
title: Never-nil comment is false
finding: Relations is omitempty, so an empty slice is omitted like nil.
severity: nit
resolution: The claim is removed; the builders return nil on no edges.
status: addressed
---
