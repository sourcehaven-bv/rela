---
id: RR-BW2Y8J
type: review-response
title: acl_query_failed response echoes raw backend error string
finding: writeGateError passes err.Error() as the V1Error detail, which can surface store/pg internals (table names, occasionally connection details in driver errors) to the client. Mirrors the pre-existing search_failed path so not a regression, but the ACL path is a new disclosure surface.
severity: minor
resolution: 'Originally deferred, then upgraded to fix-now by the CISO IB review on PR 939 (finding #1, RR-A5QEUX on TKT-VQGN). writeGateError was hardened on the base branch (constant detail "check server logs" + slog.Warn with path/method); after rebasing this branch onto that fix, writeListPipelineError received the same treatment for its list_load_failed and search_failed branches. Tests assert the synthetic backend error strings are absent from response bodies (TestACLList_QueryErrorMapping, TestACLList_AllowAllLoadErrorSurfaces, TestACLGet_WriteGateErrorMapping with a pg-shaped error).'
status: addressed
---
