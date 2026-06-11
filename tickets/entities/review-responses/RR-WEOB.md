---
id: RR-WEOB
type: review-response
title: 'Discussion: per-block opt-in vs mandatory-grant-everything (deny-by-default)'
finding: 'Crit (Jeroen): consider making the policy resolver require operators to specify all access (deny-by-default everywhere) rather than per-block opt-in — simpler code, fewer footguns. Brainstorm pros/cons.'
severity: nit
reason: 'Kept per-block opt-in. Pros of mandatory-grant (simpler code, can''t-forget-a-field) are real but outweighed: it would make every metamodel field addition silently read-only until re-granted (violates ''tolerate temporarily invalid data''), block incremental adoption (can''t lock just ticket.status without enumerating every field of every type), and diverge from how write: already defaults. Per-block opt-in already gives mandatory-grant WITHIN a governed (type,dimension); the S2 unknown-target validation closes the sharpest footgun. A per-type strict:/default_deny: flag was proposed as a future middle-ground if stronger guarantees are wanted. No code change this PR.'
status: wont-fix
---
