---
id: RR-KPQD8A
type: review-response
title: Put/Detach reload raw entity without re-checking authorized type/face; no write lock
finding: 'A rename or id reuse between the gated read and raw reload writes bytes to an entity other than the one authorized; no serialization with the web writeMu lets max be overshot. Fix: Put/Detach check the raw entity''s type and face match the authorized ones; serialize MCP writes on the App''s write mutex remotely.'
severity: minor
resolution: Put was removed. WriteAttachment and DetachFile take the entity from the gated read; the stamp patches that id and face; the write lock serializes with web writes. A rename in between makes the patch fail with an orphaned-bytes error rather than stamping another entity.
status: addressed
---
