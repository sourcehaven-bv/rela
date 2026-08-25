---
id: RR-MAILI2
type: review-response
title: Persistent filesystem claims would suppress jobs lost by the ephemeral queue
finding: A file-backed child claim survives desktop restart while the memory job queue deliberately does not. The surviving claim would prevent expansion from recreating a child that the restart discarded and turn an acknowledged ephemeral limit into guaranteed mail loss.
severity: critical
resolution: Match claim durability to queue durability. FS and desktop use in-memory claims while PostgreSQL uses durable unique claims alongside its durable queue. Both implement one conformance contract without pretending the ephemeral tier survives restart.
status: addressed
---
