---
id: RR-O547N7Z
type: review-response
title: Create pre-check answered 422 before the create authorization
finding: gateCreateRelationAffordances ran checkRelationEdges before CreateEntity authorizes the create, so a caller with no create grant got 422 (endpoint type or target_not_found) instead of 403. Found by the decl-header create.outsider/stranger goldens.
severity: significant
resolution: The pre-check now runs only when the create ACL admits the caller; a denied caller reaches CreateEntity and gets its 403. TestCreateEntity_DeniedCreateIs403BeforeEndpointType pins it and the 403 goldens are back to their develop values.
status: addressed
---
