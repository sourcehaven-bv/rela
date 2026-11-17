---
id: RR-GMMLWUF
type: review-response
title: Configure preview wrote compiled conditions into the live policy
finding: '[security] Validating a previewed acl.yaml compiled grant conditions into the RoleRelations map of the policy that live requests read: a data race and a way for an unsaved edit to change live grants.'
severity: significant
resolution: CompileGrantConditions stores a condition only where none is set; so validating a live policy writes nothing. TestValidateACLPolicy_GrantConditionsRace pins it.
status: addressed
---
