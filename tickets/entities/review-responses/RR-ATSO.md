---
id: RR-ATSO
type: review-response
title: TKT-VMD8 _position not-in-scope semantics under ACL not pinned
finding: 'If the requested id passes ACL but fails the config filter, current behavior is 404 not_in_scope. If id FAILS ACL: same 404 (don''t leak existence) — correct but not pinned. Add to AC7: ''For any requested id absent from the post-ACL post-filter scope — whether hidden by ACL, filtered by config, or genuinely not present — _position returns 404 not_in_scope with no body discrimination between the three cases.'' This is the deny-shape-parity invariant for _position, mirror of TKT-VQGN''s GET parity.'
severity: minor
reason: Carried over to the future _position ticket. The deny-shape-parity invariant for _position (404 not_in_scope for any cause) needs to land alongside the actual _position gate.
status: deferred
---
