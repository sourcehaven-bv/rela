---
id: RR-HS8L7
type: review-response
title: Action endpoint has no entity context for per-entity script invocation
finding: handleV1Action (actions.go:43-48) takes only the action ID from URL. GetEntity() and GetOldEntity() return nil. To invoke scripts per-entity from a list selection, the endpoint needs entity context. Recommend accepting entity_id/entity_type in request body — backward compatible since current calls send no body.
severity: significant
resolution: 'Plan updated: action endpoint will accept optional entity_id and entity_type in POST request body. When present, actionScriptContext will populate GetEntity()/GetOldEntity(). Backward compatible — existing sidebar calls send no body.'
status: addressed
---
