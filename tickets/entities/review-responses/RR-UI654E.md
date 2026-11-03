---
id: RR-UI654E
type: review-response
title: Compiled hands out shared mutable state
finding: ChannelType returns pointers with exported maps and slices shared across goroutines
severity: minor
resolution: Compiled accessors return deep copies (clone.go).
status: addressed
---
