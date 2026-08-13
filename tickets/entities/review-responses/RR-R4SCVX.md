---
id: RR-R4SCVX
type: review-response
title: COMPLETED without STATUS silently dropped, reverting the checkbox
finding: 'A VTODO carrying COMPLETED but no STATUS is legal RFC 5545. applyCompletionToPatch correctly treated either property as ''the client spoke about completion'', then switched on Todo.Status alone, so such a body hit the default arm and wrote nothing: 201, no warning, no log. On the next sync the server rendered STATUS:NEEDS-ACTION back and the client un-ticked the item - the same silently-reverting-checkbox symptom validateCalDAVCompletionReachable exists to prevent, reached by a route no config change can avoid. Reproduced live against the demo.'
severity: critical
resolution: 'Promote in todoFromICal: a COMPLETED timestamp with no STATUS sets Status=COMPLETED and marks STATUS as sent (the mapper keys the write on has(STATUS), so without that the promotion would be computed and never applied). Mirrors calfeed.Todo.normalized() which the outbound path already applies. Only the promotion arm is used - normalized() also demotes a STATUS:COMPLETED carrying no timestamp, which would be wrong inbound since a client legitimately sends STATUS:COMPLETED alone and the mapper stamps the time. Verified live: status=done plus completed_at now land. Regression test TestTodoFromICal_CompletedWithoutStatusIsCompletion.'
status: addressed
---
