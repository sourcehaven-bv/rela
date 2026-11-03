---
id: RR-R1U66LZ
type: review-response
title: storetest decorators do not forward Successors
finding: counting.go, bodywatch.go and breadth.go forward MentionSources but not Successors, so ResolveMentions under them silently ignores retired ids.
severity: minor
resolution: Counting, BodyWatch and Breadth decorators (and their views) forward Successors via successorsOf.
status: addressed
---
