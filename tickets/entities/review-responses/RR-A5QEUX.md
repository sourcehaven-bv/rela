---
id: RR-A5QEUX
type: review-response
title: 'IB-review PR939 #1: writeGateError echoes raw backend error in 500/504 detail'
finding: 'CISO IB-review on PR 939 (Hoog, blocks merge): writeGateError passed err.Error() as the detail field of the acl_query_failed (500) and acl_query_timeout (504) responses. PostgreSQL error strings can name tables/columns — information exposure (POLICY-015 §3 / OWASP). RR-372L applied the constant-detail + server-side slog.Warn pattern to attachACLRequest for the same reason; writeGateError was overlooked, and the test suite did not assert err.Error() is absent from the body.'
severity: significant
resolution: 'writeGateError now logs the raw error via slog.Warn (with path + method) and returns the constant detail "check server logs" on both the 500 and 504 branches — same pattern as attachACLRequest (RR-372L). TestACLGet_WriteGateErrorMapping extended: the synthetic error is shaped like a pg error naming a fake table, and the test asserts the table name is absent from the body and the constant detail is present. Same finding was independently raised by the TKT-VMD8 code review as RR-BW2Y8J (there deferred); the IB review upgrades it to fix-now on this PR.'
status: addressed
---
