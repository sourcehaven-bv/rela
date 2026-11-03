---
id: RR-YXUZ19
type: review-response
title: A retry claims that a removed account fails forbidden, but nothing checks for it
finding: 'internal/appbuild/realtime_outputs.go:65-75 rebuilds the retry principal with principal.VerifiedFrom(mark.Writer.User, ToolRealtime, Claims{PrincipalType, OrgID, Roles: mark.Roles}). It then binds through acl.BindRequest, which calls ResolveDirect, and walkMembers does not fail for a user entity that no longer exists. Global roles that came from verified claims (asserted_role_assignments) therefore still apply for up to OutputRetryWindow (30 min) after the operator deletes or deactivates the user entity. That contradicts the plan (decision 6: a removed account fails forbidden), and no test covers the case (TestOutputWriter_RetryAsPoster covers only a user without the grant). This path also adds a VerifiedFrom call site whose claims come from the channel store rather than a verified assertion, and no archguard allowlist pins VerifiedFrom call sites. Fix: when the policy sets user_entity_type, have the retry confirm that Writer.User still resolves to an existing user entity before it builds the principal, and fail forbidden otherwise. Add a test. Otherwise, drop the claim from the plan and docs and record it as part of the accepted 30-minute window.'
severity: minor
resolution: A retry whose poster's user entity is gone fails forbidden; TestOutputWriter_RetryRemovedUser.
status: addressed
---
