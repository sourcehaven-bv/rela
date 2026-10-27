---
id: RR-VW6AXT
type: review-response
title: Cursor validation is weaker than the documented contract
finding: 'internal/realtime/inbox.go:135 rejects only empty id or type. The plan and OpenAPI say that anything other than a valid entity id, face, channel type and name is 400 invalid_cursor. A cursor with `"id":"not an id"` is accepted and positions silently. This is harmless for secrecy, but the error contract is not met. Fix: check entity.ValidateID, the face, and channels.ValidName on type and name (when set).'
severity: nit
resolution: The cursor key is validated with channels.Key.Validate; four malformed cursor cases added
status: addressed
---
