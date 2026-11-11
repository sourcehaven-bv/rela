---
id: RR-CBE2K0E
type: review-response
title: Elevated update skips unmatched_principal reject
finding: 'An elevated update skips Declarative.AuthorizeWrite and with it the unmatched_principal: reject gate. A verified identity with no user entity that holds the action permission through asserted roles could write while the policy says reject (review 5815653128 finding 1).'
severity: significant
resolution: 'The checks of AuthorizeWrite that do not come from role grants now live in acl.openWrite (usable Request: no identity, foreign, mismatch; unmatched_principal) and acl.entityPreRoleDenial (client ceiling, conversion destination). Declarative.AuthorizeWrite and the new acl.AuthorizePreRoleWrite both call them, and ScopedUpdates calls AuthorizePreRoleWrite in place of its own read-only and ceiling checks, so a new pre-role check reaches elevation too. Refusals are the ACL 403 with a denied-write audit row. Role-relation requires_permission and relation_grants are role checks and stay out, and relation writes get the full AuthorizeWrite. TestAuthorizePreRoleWrite_SharesChecksWithAuthorizeWrite pins each check on both paths, and TestScopedUpdates_UnmatchedPrincipal covers reject (403, audit, row unchanged) and anonymous (elevates). The declarative.go comment and the ACL security and data-entry guides are updated.'
status: addressed
---
