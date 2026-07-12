---
id: RR-KDXGYK
type: review-response
title: Deleted-entity history must not become an existence oracle (404 vs 403)
finding: 'The live read path deliberately returns an INDISTINGUISHABLE 404 for hidden-vs-nonexistent, gating before the store read so timing can''t distinguish either (api_v1.go:870-903, visiblereader.go:57, RR-NGMI). If the history endpoint returns 403 ''history:read required'' for a deleted entity a non-holder lacks permission on, but 404 for a truly absent id, that difference CONFIRMS the deleted entity exists — an enumeration oracle over exactly the sensitive deleted set. Fix: live-entity history reuses gateReadOrNotFound/getVisible (inherits the indistinguishable-404). Deleted-entity history: a NON-holder of history:read gets the SAME 404 as a nonexistent id (never a distinguishable 403). Route all snapshot-decode/store errors through writeGateError (api_v1.go:966) so no table/column names leak. Pin with a test analogous to the RR-NGMI cases in acl_get_test.go.'
severity: significant
resolution: 'Implemented in slice 7: authorizeHistoryRead gates live-entity history via gateReadOrNotFound (indistinguishable 404); deleted-entity history requires the new global acl.PermHistoryRead, and a non-holder receives the SAME 404 as a nonexistent id (no existence oracle). Store/decode errors route through writeGateError. Tests: TestAuthorizeHistoryRead_AbsentEntityNoPermissionIs404 + holder-allowed.'
status: addressed
---
