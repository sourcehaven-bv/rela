# Provisioning users from an identity proxy (webhooks)

rela can auto-provision a `person` entity for each user of an upstream identity
proxy, driven by the proxy's webhooks. This is the companion to the signed-JWT
*identity* feature (see [server-security.md](server-security.md)): identity says
*who is making this request*; provisioning says *keep a local record of every
user, so ACLs and relations can point at it*.

The flow is **thin-notify + fetch-details**, and it is **provider-agnostic**:

1. The proxy sends rela a small **signed-JWT webhook** — "a membership changed
   for user X in org Y" — to `POST /webhooks/idp`.
2. rela **verifies** the webhook's signature (same JWKS/issuer as identity, its
   own audience) in Go, then dispatches a named **Lua action** with the claims.
3. The action **fetches** the user's authoritative data from the proxy's API and
   **upserts** a `person` entity keyed on the stable subject (`sub`).

Everything proxy-specific — which API to call, how to authenticate to it, which
JSON fields to read — lives in the **Lua action and the secrets**, not in rela's
Go. rela ships only generic building blocks (`crypto.*`, `http.*`); pointing at a
different proxy is a script + config change, no rebuild.

> User-provisioning is one *use* of two general features: the signed-webhook
> receiver (`POST /webhooks/idp` verifies a signed-JWT body and dispatches a named
> Lua action) and the `crypto.*` primitives (so an action can sign an
> authenticated outbound request). Any webhook-driven action — not just
> provisioning — is built the same way.

The reference action below targets [Pratique](https://github.com/sourcehaven-bv/pratique),
whose operator API is HMAC-signed. A different proxy would use a different action.

## 1. Add a `person` type to your metamodel

The action upserts a `person`, so the type must exist. Minimal shape:

```yaml
# metamodel.yaml
entities:
  person:
    label: Person
    id_prefix: "PSN-"
    properties:
      sub:
        type: string
        required: true   # the stable IdP subject — the match key
      email:
        type: string
```

Key on `sub`, not email: the subject is stable across email changes, so
attribution and any ACLs targeting the person survive a rename.

## 2. Add the `idp-sync` action

Copy [`examples/idp-sync.lua`](../examples/idp-sync.lua) into your project's
`actions/` directory, and register it:

```yaml
# data-entry.yaml
actions:
  idp-sync:
    label: "Sync a user from the IdP"
    script: idp-sync.lua
```

The action reads `rela.params.user_id` / `.org_id` (set by the webhook receiver
from the verified claims) and `rela.secrets` (below). It signs a Pratique
operator-API request using the generic `crypto.*` primitives — the
Pratique-specific canonical string and `X-Pratique-*` header names live entirely
in this script.

## 3. Configure the operator credential

The action fetches the user over Pratique's HMAC-signed operator API, so it needs
the base URL and the shared HMAC key:

```yaml
# .rela/secrets.yaml   (gitignored)
idp_operator_url: https://id.example.com
idp_operator_key: <the operator HMAC secret>
```

The key never touches Lua as crypto material — the script passes it to
`crypto.hmac_sha256_base64`, which signs in Go. Treat it like any other secret.

## 4. Enable the webhook receiver

Start rela with the webhook flags. They **require** the identity JWT flags
(`-jwt-issuer` / `-jwt-jwks-url`) — the webhook reuses that JWKS as its trust
root, and pins its **own** audience so an identity assertion can't be replayed as
a webhook (and vice versa):

```text
rela-server \
  -jwt-issuer   https://id.example.com \
  -jwt-audience app://your-rela \
  -jwt-jwks-url https://id.example.com/.well-known/pratique/jwks.json \
  -webhook-audience app://your-rela-webhook \
  -webhook-action  idp-sync
```

(Env fallbacks: `RELA_WEBHOOK_AUDIENCE`, `RELA_WEBHOOK_ACTION`.) With both
webhook flags set, `POST /webhooks/idp` is mounted; leave them unset and the
endpoint does not exist.

### Why the endpoint is safe without a same-origin check

`/webhooks/idp` authenticates itself by **verifying a signed JWT body** — not a
proxy-set header, not a session cookie. A browser cannot forge an ES256
signature, so the endpoint is CSRF-immune by construction. It therefore lives
outside `/api/` and needs neither the same-origin gate nor the CSRF-exempt
heuristic the [sync API](sync.md) relies on. A forged or unsigned body is
rejected with 401 before any action runs.

## 5. Point the proxy's webhook at rela

Configure Pratique to send membership webhooks to `https://your-rela/webhooks/idp`
with a JWT body whose:

- `iss` matches `-jwt-issuer`,
- `aud` matches `-webhook-audience`,
- claims include `event`, `user_id`, and `org_id`,
- and a `jti` (used for redelivery dedup — a retried webhook with the same `jti`
  is acknowledged without re-running the action).

## What you get

On each membership change, the `person` materializes (or updates) with its `sub`
and `email`. The action is idempotent, so redeliveries and repeated changes never
duplicate. Those `person` entities are then ready for ACLs and relations —
e.g. an incoming ACL feature can match the request principal's `sub` against the
`person.sub` property so policy targets the readable entity, not a raw id.

## Behavior reference

| Situation | Result |
|---|---|
| Valid webhook, user is a member | `person` upserted; 200 |
| Valid webhook, user not a member (operator API 404) | acknowledged, no `person`; 200 |
| Bad / unsigned / wrong-audience body | 401, action not run |
| Missing `user_id` claim | 400, action not run |
| Redelivered webhook (same `jti`) | deduped, action not re-run; 200 |
| Action fails (IdP fetch error, etc.) | 502, `jti` forgotten so a retry runs again |
