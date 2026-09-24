---
id: RR-30PBD2
type: review-response
title: Scope seam cannot receive candidates, gate and store
finding: QueryScopeResolver evaluates one row at a time and never sees the candidate ids, the request gate or the store; appbuild cannot name dataentry types. Carrying an answer table on ctx is a hidden channel.
severity: significant
resolution: The seam is now a batch Filter(ctx, scope, type, headers, gate, match). applyScope passes the request's traversal gate and a MatchingIDs closure; appbuild answers each distinct traversal once over the candidate ids and passes answers explicitly to the evaluator. Nothing rides on ctx.
status: addressed
---
