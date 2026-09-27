---
id: RR-S7D39S
type: review-response
title: Unsupported traversal surfaces as a generic 500 and docs overstate
finding: ErrTraversalUnsupported was wrapped in errListLoad and mapped to list_load_failed 500; docs implied the only refusal is at startup.
severity: significant
resolution: writeListPipelineError maps acl.ErrTraversalUnsupported to 422 query_scope_unsupported with the cause (config only). docs/metamodel.md lists every per-request refusal case.
status: addressed
---
