---
id: RR-PYMYTKX
type: review-response
title: Roles come from whichever org the webhook names
finding: '[security] The action kept roles when the user was active in rela.params.org_id without checking it is the deployment''s org. Pratique sends every org''s events to every endpoint and rela stores roles per subject, so a user deactivated here but active in another org got that org''s roles back.'
severity: significant
resolution: New required secret idp_org_id. An event for another org is acknowledged without asserted_roles, so it neither grants nor clears. Pinned by TestIdpSyncAction_IgnoresOtherOrg; docs sections 2 and 3 updated.
status: addressed
---
