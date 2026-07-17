---
id: RR-P6GVE
type: review-response
title: Host-function hook is specified but has no consumer in this ticket — risk of speculative/untested API
finding: 'The engine ships a pluggable host-function registry (has_role/has_relation) but the only consumer in scope (wizard forms) needs zero host functions — form conditions reference form.<field> only. Building the registry now risks a speculative API shaped by guesses about the future ACL use case, untested against a real caller (YAGNI). Two acceptable resolutions: (1) keep the registry but add at least one concrete test-only host function exercising the call path, arg passing, and error propagation, and mark the API explicitly provisional/unstable until the ACL ticket binds it; OR (2) defer the registry entirely to the ACL ticket and ship only the value-reference grammar now, leaving a clean extension point (function-call AST node parsed but rejected at eval with ''no such function''). Recommend (2) unless there is a near-term ACL consumer — it keeps this ticket minimal and avoids freezing an unproven contract.'
severity: significant
resolution: 'Decided (user): defer the registry. Parser accepts a function-call AST node (grammar stays stable) but eval rejects any call (''no such function'') until the ACL ticket adds the registry with a real caller. Keeps the API minimal and unfrozen. AC6 pins the deferred-function eval path.'
status: addressed
---
