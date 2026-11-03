---
id: RR-ERX4Z0
type: review-response
title: Every failed promote writes a released channel audit record as the actor
finding: 'internal/realtime/promote.go:180 the automatic revert uses SetAction pending to none, which writes AuditReleased by the caller, indistinguishable from a moderator release. Fix: give the revert its own audit status (reverted) or skip auditing a revert of a never-committed pending action.'
severity: minor
resolution: a promote failure that frees the message is audited with the new status reverted while a manual release stays released; channelstest asserts both (26a1cb8d8)
status: addressed
---
