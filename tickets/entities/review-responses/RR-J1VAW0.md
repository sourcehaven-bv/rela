---
id: RR-J1VAW0
type: review-response
title: Paging pushdown must ride the per-request GraphQuery copy and expose only the scoped count
finding: pushdown.go:117-124 copies *rqr.Query because the ACL layer reuses the compiled result per principal. New OrderBy/Limit/Offset fields set on the shared value would bleed one request's page into another caller's. GraphCount returns (matched, total); the handler holding both can trivially return the unscoped total, which is the RR-SSPCCI count oracle.
severity: significant
resolution: Plan states that OrderBy/Limit/Offset are set only on the shallow copy made in listPushdown, with a test mirroring the existing world-bleed test; the list handler receives only the matched count from GraphCount via a helper that does not return total, and a test asserts a scoped principal's response total equals the visible count.
status: addressed
---
