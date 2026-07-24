---
id: RR-9V6PF
type: review-response
title: Host-fn regex/glob patterns compiled per-Eval, validated only at Eval
finding: 'matchRegex/matchGlob call regexp.Compile/filter.ParsePattern inside the func body every call, no caching. For an ACL predicate evaluated per-entity, an identical literal pattern recompiles O(entities) times, and a malformed literal pattern errors on every eval rather than being rejected once at policy load. Both RE2 so no ReDoS. Fix (Phase 2): compile-time validation of constant-literal patterns + a per-Program pattern cache. Deferred to Phase 2 (needs Program-level cache infra).'
severity: minor
reason: Per-Eval pattern recompilation + compile-time pattern validation needs a per-Program cache / constant-folding pass that doesn't exist yet. Not a correctness or security issue (RE2 = no ReDoS; malformed pattern errors safely, just repeatedly). Deferred to Phase 2 where the Program-level infrastructure for caching constant-literal patterns lands alongside the CLI --filter compile-once path.
status: deferred
---
