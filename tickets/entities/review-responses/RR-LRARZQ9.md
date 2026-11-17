---
id: RR-LRARZQ9
type: review-response
title: Configure Validate passes the served policy to LoadRealtime
finding: Validate checks flows against a fresh policy copy but LoadRealtime still gets svc.ACLPolicy().
severity: nit
resolution: LoadRealtime only reads role names from the policy and does not change it; the copy is needed only where validation expands the policy in place. Unchanged by this ticket.
reason: LoadRealtime only reads role names from the policy and does not change it; the copy is needed only where validation expands the policy in place. Unchanged by this ticket.
status: wont-fix
---
