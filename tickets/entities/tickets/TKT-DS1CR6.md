---
id: TKT-DS1CR6
type: ticket
title: 'Mail extensibility: HTTP + Lua script transports, mail.send binding, base64/multipart/basic-auth primitives'
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Third of three, after the SMTP foundation (TKT-332QZY) and the declarative layer
(TKT-U2R7GU). Opens mail to providers rela does not ship support for, and
exposes sending to scripts.

Deferred to last deliberately: SMTP covers the common deployment, and the
declarative layer is where the user value is. Extensibility is worth building
once both are real, so the `Sender` interface is shaped by two working consumers
rather than by speculation.

## Scope: IS

**`transport: http` — one Go transport.** SimpleMailService APIv2, verified
against live docs: `POST
https://api.simplemailservice.eu/v2/accounts/{ACCOUNT_ID}/messages`,
`Authorization: Bearer <token>`, fields `from{email,name}`,
`recipients[]{email,name}`, `subject`, `html_content`, `text_content`,
`attachments[]{data,base64,content_type,file_name}`, `substitutions{}`.

**`transport: script` — the general answer.** Provider APIs diverge on every
axis, so no JSON field-mapping DSL can cover them:

| Provider | Encoding | Auth | Sender | Recipient | HTML | Text |
|---|---|---|---|---|---|---|
| SimpleMailService | JSON | `Authorization: Bearer` | `from{email,name}` | `recipients[]{email,name}` | `html_content` | `text_content` |
| Postmark | JSON | `X-Postmark-Server-Token` | `From` (string) | `To` | `HtmlBody` | `TextBody` |
| Resend | JSON | `Authorization: Bearer` | `from` (string) | `to` | `html` | `text` |
| Mailgun | **multipart/form-data** | HTTP Basic (`api:KEY`) | `from` | `to` | `html` | `text` |

Mailgun is not JSON at all (confirmed from `mailgun-go`'s own
`FormDataPayload`), so a mapping DSL would silently exclude it. Instead an
operator Lua script receives an already-rendered message and ships it. Example
scripts for Mailgun / Postmark / Resend live in `examples/`, **not** compiled in
— the FEAT-CN5L0X precedent (generic primitive in Go, provider specifics as
example Lua).

**Lua primitives — general, not mail-specific.** Today `http.*` accepts only
`url`, `method`, `headers`, `body` (string) and `timeout`, and there is **no
base64 anywhere** in the runtime (`crypto` has `sha256_hex` and
`hmac_sha256_base64` only; gopher-lua's `string` lib has none). So JSON
providers are reachable from Lua today but form-encoded ones are not. Add:

- `crypto.base64_encode` / `crypto.base64_decode`
- `http` `form = {…}` (multipart/form-data) and `basic_auth = {user=…, pass=…}`

Without these, "bring your own backend" is true only for JSON providers.

**`mail.send{...}` Lua binding** — free-function `registerMailModule` (crypto.go
style; `Runtime` is at its `//plimsoll:max-methods=120` ceiling),
`SetGlobal("mail", tbl)`, registered unconditionally so scripts can
feature-detect, with a `not_configured` guard inside the binding. Network-bound,
so it returns `(nil, err_table)` per the `ai.*`/`http.*` convention — never
`RaiseError` for a delivery failure. Wiring goes through
`lua.LoadContextOptions` (a `WithMailSender` Option), the single sanctioned load
point — not a new `LoadProvider`-style call site.

## Depends on TKT-YH52OM (PR #1385)

That ticket lands fail-closed `lua.Capabilities` —
`http`/`ai`/`secrets`/`write_file` must be explicitly declared, `secrets` as a
**list of key names** rather than a bool. Its motivation is exactly the risk a
send script poses: *"a script could read a secret and POST it anywhere in two
calls"* — which held even in read-only runtimes.

So the send script gets no bespoke sandbox. It gets the shipped mechanism with
the narrowest grant and **no graph deps at all**:

```yaml
transport: script
script: mail/mailgun.lua
capabilities:
  http: true
  secrets: [mailgun_key]     # a list, never a bool
```

The runtime is built with no `ReadDeps`/`WriteDeps`, so a send script **cannot
read or write the graph** — it receives a rendered message and can only ship it.
Enforced by construction, not asserted in prose. Rendering is ACL-gated upstream
(TKT-U2R7GU), so the script never needs graph access to do its job.

By the time this ticket starts, #1385 should be merged; confirm the
`Capabilities` API matches before building on it.

## Open question — DECIDED

**Question.** The outbox worker has no triggering principal or script context —
it delivers minutes after the fact, on retry, with no `run_as` and no
script-path-keyed secrets scope (`secrets.Load` is keyed by script path). How
does the send runtime obtain its secrets scope and audit identity?

**Decision.** A fixed `system:mail` principal (plus tool `mail`), with the
secrets scope keyed on the **configured script path** — the value of `script:`
in mail.yaml, as written, not resolved to an absolute path. Recorded in
`mail.SendScriptPrincipal`'s godoc and pinned by
`TestScriptSender_Principal` / `TestScriptSender_PerScriptSecretOverride`.

**Reasoning.** The alternative was to carry the enqueuing principal on the
Message and run the script as them. It reads like better attribution and is
worse on every axis that matters:

- The credential the script uses is the OPERATOR's, not the user's. An audit
  line naming a person with no relationship to that credential — and no ability
  to revoke it — is worse than no attribution, because it is misleading rather
  than merely absent.
- A per-user secrets scope would make delivery succeed or fail depending on who
  happened to trigger it. That is a nondeterministic mail system.
- `Message.RenderedFor` already records the identity that matters: the one
  whose visibility bounded the CONTENT. Attribution of the DELIVERY is a
  different question, and conflating them loses the second — which is the one
  the ACL model depends on.

Positively: delivery is infrastructure the operator configured once, in a file
only they can write, and every send through a given transport is the same act
whatever triggered it. The script path is the right scope key because it is
what `secrets.Load` already keys on everywhere else, so an operator writes the
ordinary `overrides:` block and it works — no second, mail-only convention.
The path is also stable across restarts and retries, which a principal is not.

The path is used *as written* rather than resolved, because `overrides:` keys in
secrets.yaml are project-relative script paths; scoping on an absolute path
would never match an override and the operator's per-script credential would be
silently ignored.

## Scope: IS NOT

- No Mailgun/Postmark/Resend **Go** transports — those are example scripts.
- No JSON field-mapping DSL (see the table above for why).
- No inbound mail, bounce handling, or open/click tracking.
- No durable queue (IDEA-WIJ2H1).

## Acceptance criteria

1. `transport: http` delivers against an `httptest.Server` asserting the exact APIv2
path, `Authorization: Bearer`, and the body field names above.
2. `transport: script` delivers via a Lua script against an `httptest.Server`.
3. All transports (smtp, memory, http, script) satisfy the shared conformance suite
introduced in TKT-332QZY, unchanged.
4. A send script's runtime has no graph access: `rela.get_entity` /
`rela.list_entities` are absent, and an undeclared secret is `nil` — asserted,
not assumed.
5. The shipped `examples/mail/mailgun.lua` posts multipart/form-data with Basic auth
against a stub asserting Mailgun's exact field names — proving the new
primitives against a real-world shape.
6. `crypto.base64_encode` / `_decode` round-trip, with known vectors.
7. `http` `form` produces a well-formed multipart body; `basic_auth` produces the
correct header.
8. `mail.send` from a script with the capability declared succeeds; without it, the
binding is absent and the failure is loud, not a silent no-op.
9. A script transport failure surfaces as a typed err_table and retries through the
existing outbox backoff without duplicating.
10. Credential never appears in logs, errors, or `/api/v1/_config`.

## Risks

- **`Sender` reshape** — if TKT-332QZY's interface assumed an in-process transport,
this ticket pays for it. Mitigated by sanity-checking the interface against this
sketch during that ticket's planning.
- **Exfiltration via a send script** — narrowed by capability gating and the
no-graph-deps runtime; the `internal/ai` threat model ("treat Lua scripts as
trusted code") still applies and should be restated in the package doc.
- **Worker-context gap** — see the open question above; unresolved at filing time.
- **Example-script rot** — shipped examples target third-party APIs that change.
They are tested against local stubs, which pins *our* contract but not the
providers'. Say so in the docs rather than implying they are supported
integrations.

## Implementation notes

**Capabilities API confirmed.** `lua.Capabilities` matches what this ticket
assumed: `HTTP`/`AI`/`WriteFile` bools plus `Secrets []string`, fail-closed
zero value, `AllSecrets` reserved for the operator-shell boundary and not
settable from YAML. `mail.ScriptCapabilities` is a narrower YAML face over it —
`http` and `secrets` only, with `AI`/`WriteFile` hard-wired false, because a
mail transport has no business spending money on inference or touching the
disk, and a field that exists in YAML is a field someone eventually sets.

**Runtime's plimsoll ceiling moved 105 -> 106.** The ticket predicted the
mail.send binding would have to be a free function to stay under the load line.
`registerMailModule` IS a free function, as specified. The binding itself had to
become a method value: `contextcheck` follows a registration CLOSURE back
through `registerBindings` to all twelve `NewReader`/`NewWriter` call sites and
demands each thread a context into runtime CONSTRUCTION, which is not a thing
that exists — a binding runs later, when a script calls it. `ai.*` and `http.*`
are unflagged only because they register method values. Matching them is the
substantive fix; suppressing the finding at twelve unrelated call sites would
have been the cosmetic one. The directive carries that reasoning.

**`SenderFor` hoisted into internal/mail.** The transport switch was duplicated
in `internal/appbuild`; with four transports a two-copy switch over a closed set
is how one gets wired in one place and not the other.

**Wiring.** `lua.LoadContextOptions` gained a `MailSenderLoader` parameter and
`internal/script` supplies `mail.LoadLuaSender`. The loader is a parameter
rather than a direct import because `internal/mail` depends on `internal/lua`
(transport: script runs a Lua runtime), so the reverse import would be a cycle.
`LoadContextOptions` stays the single load point — no parallel `LoadProvider`-
style call site was added, per the rule internal/ai already states.
