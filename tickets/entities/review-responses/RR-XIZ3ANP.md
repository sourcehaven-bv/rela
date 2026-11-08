---
id: RR-XIZ3ANP
type: review-response
title: Redis switch from Run to Start changes signal handling
finding: Run registered signal.Notify; with Start an embedding app no longer gets asynq self-shutdown on SIGTERM and must call Shutdown.
severity: minor
resolution: Stated in the fork PR and the rela PR body. rela does not use the redis backend.
status: addressed
---
