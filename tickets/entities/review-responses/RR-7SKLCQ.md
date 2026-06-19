---
id: RR-7SKLCQ
type: review-response
title: ApplyRelation drops the finite-order guard, not just auto-assignment
finding: 'CreateRelation/UpdateRelation don''t only auto-assign managed order — they also REJECT garbage values (validateOrderUpdate/FiniteOrder guards, manager_order.go:91-117) because direct write paths (MCP/Lua/CLI) can submit junk. ApplyRelation is another direct path and persists whatever _order_out/_order_in it carries with zero validation, reintroducing the bug class the guards prevent (e.g. _order_out: ''NaN'').'
severity: significant
resolution: Accepted as a documented trust decision (not silently dropped). The ApplyRelation godoc now states sync mirrors whatever finite order the origin already assigned through its normal (guarded) write path; the sync caller is trusted to carry well-formed order from a peer. If untrusted payloads become a concern, re-running the finite-order backstop here is the follow-up.
status: addressed
---
