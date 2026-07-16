---
id: RR-UOBUC
type: review-response
title: Guard adapter fails OPEN when acl.Request is absent on a served path
finding: 'transitionGuard.HoldsPermission (transitions.go:27-33) returns true when acl.FromContext(ctx)==nil. The guard is the finer gate (establish != update); ''top-level write ACL already ran'' does not cover it. If a served path ever loses its Request (new endpoint missing middleware, background job reusing a ctx, a future refactor), every guarded transition silently opens - the exact escalation the guard prevents. The rest of the ACL fails closed (router 500s on unresolved principal; walk aborts rather than over-grant). Fix: wire the adapter with the resolved policy at build time; fail CLOSED when a policy exists but the Request is missing, stay inert only when there is genuinely no policy.'
severity: significant
resolution: 'transitionGuard now carries policyActive (set true when resolvedACL is *acl.Declarative). When the acl.Request is absent: inert (allow) only if no policy is active; fails CLOSED (deny) when a policy exists but the Request is missing — matching the ACL''s fail-closed posture. CompileTransitions takes resolvedACL to determine this; both wiring sites (appbuild, fixture) updated.'
status: addressed
---

## Finding

`transitionGuard.HoldsPermission` (transitions.go:27-33) returns **true** when
`acl.FromContext(ctx) == nil`. The doc justifies this as "the top-level write
ACL already ran" — but that gate authorizes the coarse `update` verb, while the
guard is the *finer* gate (`establish` is a distinct permission a mere-updater
may lack). They gate different things.

If a served path ever loses its `acl.Request` (a new endpoint that forgets the
middleware, a background job reusing a request ctx, a future refactor of the
Request-attach), every guarded transition silently opens — the exact escalation
the guard exists to stop.

The rest of the ACL is emphatically fail-**closed**: the router 500s on an
unresolved principal; member/ancestor walks abort rather than over-grant;
`RenameEntity` fails closed on a flaky store read. This adapter is the one
fail-open spot, and it's the security-sensitive one.

## Resolution

The "inert on CLI/no-policy" behavior is legitimate. The fix is to distinguish
"no policy configured" (inert, allow) from "policy exists but Request absent"
(fail closed). Wire `transitionGuard` with the resolved
policy/`*acl.Declarative` at build time; fail closed when a policy exists and
the Request is missing.
