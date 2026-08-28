---
id: RR-CTX4
type: review-response
title: Cascade denial named the wrong endpoint
severity: significant
status: addressed
finding: 'For an incoming edge rel.To IS the entity being deleted, so the error read ''cannot delete REQ-1:
  its addresses relation to REQ-1''. That is nonsense, and it withheld the far endpoint whose type actually
  blocked the delete. docs/acl-overview.md documented the correct output; the code did not match it.'
resolution: Split the loops by direction so the message names the far endpoint. The test now asserts DEC-1
  and 'incoming' appear, which is the assertion that would have caught it.
---
