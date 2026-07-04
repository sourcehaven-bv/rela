---
id: RR-4C2AI4
type: review-response
title: '"Read-only script.Engine path" does not exist; ACL scoping comes from the read gate, not a read-only runtime'
finding: |-
    The plan repeatedly says the feed script runs 'through the read-only script.Engine path' and treats that as the ACL-scoping mechanism. Verified in internal/script/executor.go: every Engine entry point (ExecuteCode/ExecuteFile/ExecuteDocument/ExecuteAction) takes `lua.WriteDeps` and builds a writer runtime via NewWriterRuntime. There is NO read-only execution path in script.Engine. ACL scoping does NOT come from runtime read-only-ness — it comes from (a) the ACL read gate attached to the request context by attachACLRequest (router.go:152) and (b) the ACL-aware store honoring that gate on reads.

    CONSEQUENCES / FIX: (1) Correct the plan's wording — the feed handler runs a writer runtime like every other script, and safety rests on the read gate + ACL store, not a read-only runtime. (2) Decide deliberately whether a feed script should be allowed to WRITE (it can, since it gets WriteDeps). A feed is conceptually a read; consider running it with a WriteDeps whose Mutator is nil/deny, OR document that feed scripts may mutate and are audited like any write. (3) Ensure the request-context read gate is actually attached on the feed path: attachACLRequest currently wraps the `/api/` inner mux — confirm the feed route sits inside that wrap so `list_entities` in the feed script is ACL-scoped. If the feed handler is registered on the outer mux (like SSE), it BYPASSES the read gate and leaks. This is the single most important wiring detail for AC10.
severity: significant
resolution: 'Plan updated (PLAN-6LOL0Z §3, Security): corrected wording — there is no read-only script.Engine path; every Engine entry takes WriteDeps. ACL scoping comes from the request-context read gate (attachACLRequest on the inner /api/ mux) + the ACL store, so the feed route MUST register on the inner mux (RR-4AWSTN). Feeds run a writer runtime with a write-denying Mutator (a feed that mutates is a bug).'
status: addressed
---
