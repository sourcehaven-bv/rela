---
id: RR-ZVUHXO
type: review-response
title: 'webhook: is a grantable identity namespace but not reserved — assertable from the wire'
finding: '[security] webhook_routes.go:157-160 mints principal.Principal{User: "webhook:" + hookID}. A hook is granted write rights in acl.yaml — the only way to give it any — so `webhook:icinga-alert` is a grantable identity carrying real write permissions. But principal.IsReserved tests only ReservedPrefix = "system:", so stampAuditPrincipal (router.go:824) does not refuse a request that ASSERTS that name. Verified: IsReserved("system:scheduler")=true, IsReserved("webhook:icinga-alert")=false. This reproduces the exact hazard the IsReserved godoc documents for system: — the ACL resolves principals by plain map lookup on the raw string with no notion of provenance, so it cannot distinguish a forged webhook identity from the real one. Attack: operator grants webhook:icinga-alert a write role; a deployment using --principal-header sends X-Rela-User: webhook:icinga-alert to /api/v1/incidents and inherits the hook''s grants on the normal API surface. Inverted, an audit row naming webhook:icinga-alert can no longer be trusted to have come from /hooks/. Fix: mint as system:webhook:<hookID>, which inherits the existing refusal, docs and grantability semantics for free.'
severity: significant
resolution: The webhook principal is now minted as principal.ReservedPrefix + "webhook:" + hookID, i.e. system:webhook:<id>. That inherits principal.IsReserved's existing refusal at every request-path entry point rather than adding a second boundary someone must remember, and it inherits the existing docs and grantability semantics unchanged. docs/webhooks.md updated to name the new form and say why the prefix is there.
status: addressed
---

[security] `internal/dataentry/webhook_routes.go:157-160` mints
`principal.Principal{User: "webhook:" + hookID}`. Because a hook is granted
write rights in `acl.yaml` — the only way to give it any —
`webhook:icinga-alert` becomes a **grantable identity carrying real write
permissions**.

But `principal.IsReserved` tests only `ReservedPrefix = "system:"`, so
`stampAuditPrincipal` (`router.go:824`) does not refuse a request that *asserts*
`webhook:icinga-alert`. Verified directly:

IsReserved("system:scheduler")     = true IsReserved("webhook:icinga-alert") =
false

This reproduces exactly the hazard the `IsReserved` godoc documents for
`system:`:

> a reserved name arriving from the wire is identity spoofing that inherits the
> scheduler's grants; the ACL cannot detect it, because it cannot tell a forged
> `system:scheduler` from the real one.

The ACL resolves a principal by a plain map lookup on the raw string
(`policy.Assignments[user]`) and has no notion of provenance. The webhook
namespace creates the same condition and omits the same check.

**Attack path.** Operator grants `webhook:icinga-alert` a `write: [incident]`
role. Deployment uses `--principal-header X-Rela-User` behind a proxy (a
documented mode). A caller who reaches rela past the proxy — or an IdP that
permits self-asserted `sub`, or a proxy misconfiguration — sends `X-Rela-User:
webhook:icinga-alert` to `/api/v1/incidents` and inherits the hook's grants on
the *normal* API surface. Inverted, it also means an audit row reading
`webhook:icinga-alert` can no longer be trusted to have originated at `/hooks/`.

**Fix.** Mint the principal as `system:webhook:<hookID>` — it inherits the
existing refusal, the existing docs and the existing grantability semantics for
free, with no new mechanism. Alternatively extend `IsReserved` to a
`reservedPrefixes` set including `webhook:`. Update the reserved-identity table
in `docs/server-security.md` either way.
