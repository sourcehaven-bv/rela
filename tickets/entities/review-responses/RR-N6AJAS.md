---
id: RR-N6AJAS
type: review-response
title: Header allowlist floor omits the deployment's own configured principal header
finding: '[security] forbiddenWebhookHeaders (validate_webhooks.go:30-46) is a static literal list whose comment claims it covers ''rela''s own principal header''. But rela''s principal header is operator-configured to an ARBITRARY name via --principal-header (cmd/rela-server/main.go:88; docs/server-security.md:168 says ''or any header name''). The validator has no access to that value. X-Forwarded-User is covered only because it is the documented example; any other choice is unprotected. Attack: operator runs --principal-header X-Pratique-User, configures a hook with headers: [X-Pratique-User] and a step set: {reported_by: ''{{header.x-pratique-user}}''} intending ''record who filed it''. The proxy-asserted identity of any authenticated user is now persisted into entity content and served back on every read, to every principal who can read it, with no visible: redaction and no indication it came from an auth header rather than the payload. The allowlist is opt-in so an operator must write the name — but that is exactly the always-a-mistake class the floor exists to stop, and the floor silently does not cover it. The mechanism is otherwise correct (allowlist not passthrough, case-insensitive, RFC 7230 token grammar). Secondary gap: the family is incomplete — x-authentik-username, x-authentik-email, x-forwarded-groups, x-vouch-user, x-pomerium-claim-*, x-remote-email, x-remote-groups. Fix: thread the configured principalHeader into webhook validation so it is refused alongside the static list; broaden the static family; consider refusing x-forwarded-*, x-auth-request-*, x-remote-* by PREFIX rather than exact name.'
severity: significant
resolution: 'Three changes. (1) Prefix-based refusal (forbiddenWebhookHeaderPrefixes): x-forwarded-, x-auth-request-, x-remote-, x-authentik-, x-pomerium- — chosen over extending a name list that had already proven incomplete once, so it fails safe for the next proxy nobody has heard of. (2) Exact names added for the authentik and vouch families. (3) dataentryconfig.ForbidWebhookHeader(name) lets the wiring site register a header only knowable at startup; App.SetPrincipalHeader now calls it, so the deployment''s own -principal-header is refused whatever it is called. The registration deliberately lives in dataentry rather than cmd/rela-server — arch-lint forbids cmdServer depending on dataentryconfig, and SetPrincipalHeader already receives the name. Covered by TestValidateWebhooks_ForbiddenHeaderFamilies (including a benign header that must still be allowed, or the rule would be useless) and TestForbidWebhookHeader_RegistersDeploymentPrincipalHeader.'
status: addressed
---

[security] See `finding`.

The mechanism is right — allowlist rather than pass-through, case-insensitive
compare, RFC 7230 token grammar on the name. The gap is purely in the
completeness of the *floor*: the one header guaranteed to carry an authenticated
identity in a given deployment is the one the validator cannot see, because it
is a startup flag rather than a constant.

`--principal-header` is known at startup and `validateWebhooks` runs at config
load, so threading it in is the same shape as the metamodel the validator
already receives — or a package-level `forbidHeader(name)` registration from the
wiring site.

Prefix-based refusal (`x-forwarded-*`, `x-auth-request-*`, `x-remote-*`) would
close the residual family gap more durably than extending a name list that has
already proven incomplete once.
