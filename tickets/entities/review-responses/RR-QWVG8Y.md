---
id: RR-QWVG8Y
type: review-response
title: gatedAuthorizer must not derive from the ctx read gate alone — nopReadGate is permissive under BOTH NopACL and ReadOnly
finding: The read gate reached via readGateFromContext(ctx) returns nopReadGate under BOTH NopACL and ReadOnlyACL (readgate.go; commands_test.go:1048-1051 documents this exactly), and nopReadGate.HoldsPermission returns true. So a gatedAuthorizer whose Authorize() consults only the ctx read gate would FAIL OPEN under ReadOnlyACL and under NopACL — which is precisely the RR-CWWJGW bug authorizeCommand's type-switch exists to prevent. The plan's safety therefore rests ENTIRELY on the wiring-site selection choosing denyAuthorizer for ReadOnly and ungated/deny for NopACL, so that gatedAuthorizer is ONLY ever constructed when a *acl.Declarative is present (whose per-request gate genuinely distinguishes held/unheld). The plan implied this but did not state it as load-bearing. gatedAuthorizer must be constructed with an explicit non-nil *acl.Declarative dependency and refuse to exist otherwise; it must NEVER be the fallback for an unknown/nop ACL.
severity: critical
resolution: 'Plan updated: gatedAuthorizer is constructed ONLY from a concrete non-nil *acl.Declarative (mirrors scriptEntityReader''s `if d == nil` guard). selectCommandAuthorizer maps ReadOnly->deny and NopACL->ungated/deny BEFORE any gated path, and gated is unreachable without a Declarative. Added an explicit AC + test: ReadOnlyACL must 403 even though its ctx read gate answers permissive (the RR-CWWJGW canary, preserved verbatim). Documented in the plan that the ctx read gate cannot self-distinguish NopACL vs ReadOnly — the wiring seam is the distinguisher, by design.'
status: addressed
---
