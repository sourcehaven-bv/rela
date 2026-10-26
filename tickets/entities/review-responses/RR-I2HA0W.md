---
id: RR-I2HA0W
type: review-response
title: Pre-check should use Manager.AuthorizeUpdate, not a new grants-only CheckUpdate
finding: 'The plan adds a package function `CheckUpdate` that answers update "from the grants, without row reads". Two problems. (1) internal/entitymanager/readgateguard_test.go (TestAuthorizeAndAuditFollowsReadGate) fails any function that calls authorizeAndAudit with no read gate before it; CheckCreate is exempt only because a create names no row (readgateguard_test.go:87). (2) An update decision for an existing row includes local roles (role_relations, inherit_roles_through) and conditions. Research decision 4 puts session roles on graph properties, so a DPO with update only through a local role would get a false 403 `session_output_denied`. The method already exists: Manager.AuthorizeUpdate (internal/entitymanager/manager.go:1306) runs the read gate, then the real update question, and returns an UpdateGrant. Plan change: drop CheckUpdate. Build the row from the anchor the call already read (c.anchor: type, id, face) and call AuthorizeUpdate. Accept its denied-write audit row: a refused post is a real refused attempt. Optionally use UpdateGrant.PatchEntity for the inline write. Remove the claim "decided from grants before any row read" from Security.'
severity: significant
resolution: Dropped CheckUpdate. The pre-check calls Manager.AuthorizeUpdate on the anchor row the call read, and its denied-write audit row is accepted. The inline set uses UpdateGrant.PatchEntity. The Security claim is removed.
status: addressed
---
