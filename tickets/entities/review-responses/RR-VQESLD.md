---
id: RR-VQESLD
type: review-response
title: ACL-mode guard does not cover Declarative assertions
finding: archguard aclmode_test flags NopACL and ReadOnlyACL only; 'is a policy configured' is still a *acl.Declarative assertion in dataentry, entitymanager and the watcher, and those sites treat a typed-nil Declarative differently from PolicyConfigured.
severity: significant
resolution: Mode godoc now states the guard scope and that the Declarative assertions move behind Mode in a later TKT-WFS778 step, when the guard widens. Migrating them is outside the five switches this ticket owns.
status: addressed
---
