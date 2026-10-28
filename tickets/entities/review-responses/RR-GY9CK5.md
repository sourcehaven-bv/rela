---
id: RR-GY9CK5
type: review-response
title: Person-id form assumes principal.User is the user entity id whenever user_entity_type is set
finding: 'The plan switches to the entity-id form (and turns `@login` off) on `user_entity_type` alone. The web principal is only re-stamped to the entity id when the principal_property lookup is enabled AND matches: access.Opener -> acl.Policy.PrincipalPropertyLookupEnabled needs both user_entity_type and principal_property (internal/acl/policy.go:330), and a failed or unmatched lookup falls back to the raw login (opener.go resolveIdentity). In a valid config with user_entity_type but no principal_property, every person''s key is their login, so a `USR-7` mention writes a row nobody''s ListAddressed will ever read, `@login` is plain code, and the picker inserts ids that notify no one. The failure is silent. Fix: decide the deployment kind on the lookup flag, as appbuild.pilePersonType already does: entity-id form only when both keys are set; otherwise the `@login` form. Put that one predicate on Compiled (say MentionUserType()) and use it in classifySpans, render, the schema block (login_form) and the picker. Document that a principal the lookup does not match (a break-glass UPN) can be reached by neither form, and add a TestClassifySpans row for user_entity_type without principal_property.'
severity: critical
resolution: Added Compiled.MentionUserType() (user type only when principal_property is also set, as pilePersonType); used by classify, render, docs. AC 2 row without principal_property; break-glass logins documented.
status: addressed
---
