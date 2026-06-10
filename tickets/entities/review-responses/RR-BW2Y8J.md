---
id: RR-BW2Y8J
type: review-response
title: acl_query_failed response echoes raw backend error string
finding: writeGateError passes err.Error() as the V1Error detail, which can surface store/pg internals (table names, occasionally connection details in driver errors) to the client. Mirrors the pre-existing search_failed path so not a regression, but the ACL path is a new disclosure surface.
severity: minor
reason: 'Deferred: writeGateError''s shape is TKT-VQGN (PR 1) surface, decided there under RR-89XK, and the identical err.Error()-as-detail convention is used by search_failed, list_load_failed, and other v1 error paths — sanitizing only the ACL branch would be inconsistent and incomplete. The right fix is a one-pass hardening ticket that logs details server-side and returns generic wire details across all v1 5xx paths; tracked as a follow-up to file when TKT-VMD8 lands. rela-server''s threat model (loopback-default, trusted-proxy deployments) keeps the immediate exposure low.'
status: deferred
---
