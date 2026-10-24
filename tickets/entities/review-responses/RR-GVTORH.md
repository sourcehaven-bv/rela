---
id: RR-GVTORH
type: review-response
title: ListSessionsDue tie order and limit contract are underspecified
finding: 'The contract says ''ordered by NextDueAt then key'', but the SQL sketch orders by session_next_due_at, id, which is the insertion row id, not the key. A conformance test with ties will disagree across backends. limit <= 0 is also undefined. Plan change: order SQL by session_next_due_at, anchor_id, anchor_face, type, name (or declare ties unordered and do not test them), define limit <= 0 as ErrInvalidMessage, and add both cases to testListSessionsDue.'
severity: minor
resolution: 'Plan updated: SQL orders by session_next_due_at, anchor_id, anchor_face, type, name; limit <= 0 is ErrInvalidMessage, empty types ErrInvalidKey; testListSessionsDue covers ties and both errors.'
status: addressed
---
