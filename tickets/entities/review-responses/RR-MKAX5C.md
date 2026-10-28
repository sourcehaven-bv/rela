---
id: RR-MKAX5C
type: review-response
title: Comment polling cost on the remote server
finding: A list_comments target replays every 2s without pacing; each replay is a gated read plus a body parse.
severity: minor
reason: Bounded by the existing watch limits (16 targets per watch, 4 per identity, 32 per server, 4 parallel replays). The use case is stdio, where the poll is per process. An in-process wake from comments.Service can replace the remote poll if load shows up.
status: deferred
---
