---
id: RR-CWBZVT
type: review-response
title: Late-binding tests reassign app.acl after construction; a wiring-time authorizer won't track that — must be resolved
finding: 'commandHandler.aclImpl is deliberately a closure `func() acl.ACL { return app.acl }` (app.go:664) BECAUSE tests reassign app.acl after construction (commands_test.go:1061,1102,1197,1229,1258,1267 all do `app.acl = ...`). If the authorizer becomes a wiring-time value chosen from the ACL-at-construction, those tests would silently exercise the WRONG authorizer — they''d set app.acl=ReadOnlyACL but the handler would still hold whatever authorizer NewApp was given (default ungated), so TestCommandExecReadOnlyDenied and the pointer-ReadOnly cases would FALSELY PASS while testing nothing. This is a test-integrity hazard, not just churn: a security canary that passes vacuously is worse than none.'
severity: significant
resolution: 'Plan updated: the late-binding tests are migrated to inject the authorizer directly (set app.commands.authz = denyAuthorizer{} / gatedAuthorizer{...} / ungatedAuthorizer{}) instead of reassigning app.acl. Alternatively, if any test genuinely needs to drive the selection through the ACL, it calls selectCommandAuthorizer(app.acl, bind, override) and assigns the result. Added AC7-adjacent note: no test may set app.acl expecting command auth to follow; a lint/grep guard flags `app.acl =` in command tests. Confirmed only 3 production NewApp call sites (docscapture/server.go:97, rela-desktop:170, rela-server:383) so the trailing-param churn is small and mechanical.'
status: addressed
---
