---
id: RR-H3HMCF4
type: review-response
title: last_reply_at could move backwards
finding: A reply with an earlier clock lowered the parent's last_reply_at.
severity: minor
resolution: SQL takes the later of the two; the file store keeps the parent's time when later. Conformance test LastReplyAtNeverBack.
status: addressed
---
