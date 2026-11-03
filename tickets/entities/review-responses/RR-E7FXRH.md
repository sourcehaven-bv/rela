---
id: RR-E7FXRH
type: review-response
title: Retry identity fails for SSO users and loses resolution details
finding: 'Removing token roles and scopes only narrows the identity (principal.go:66-83), so it cannot escalate. But in a deployment where data-entry users get their grants through asserted_role_assignments, every retry and every mark deferred under a degraded snapshot fails `forbidden`. For those deployments the runner path for message-driven marks does nothing useful. The plan also does not say which resolver the runner uses. Author.User is already the resolved entity id after substituteUser (acl/identity.go:150-164), so it must go through ResolveDirect, not Resolve, and RawUser is lost from the audit. Plan change: name ResolveDirect. Decide between storing the verified role claim names in the mark with a validity bound (for example the attempt window) and documenting that SSO-role users get no retry. Under a degraded snapshot, refuse a post that would fire outputs rather than defer it to a narrowed identity.'
severity: minor
resolution: Retries use ResolveDirect. The mark stores verified role claims, used only within a 30 min OutputRetryWindow, then the mark fails expired. Scopes are not stored. Posts that would fire outputs while degraded are refused with 403.
status: addressed
---
