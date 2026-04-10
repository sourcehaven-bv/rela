---
id: RR-IPEN3
type: review-response
title: Bulk set actions trigger automations per entity — document this
finding: UpdateEntity (workspace.go:930-935) runs automation.Process() on every PATCH. A set action changing status on 10 entities triggers automations 10 times. This is correct behavior but should be documented. The confirm flag naturally gates heavy-automation actions.
severity: nit
resolution: Will document in ticket design that bulk actions trigger automations per entity. The confirm flag is the natural mitigation. No code change needed.
status: addressed
---
