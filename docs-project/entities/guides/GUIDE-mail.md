---
audience: intermediate
id: GUIDE-mail
order: 12
status: published
summary: Configure outbound email — SMTP, branding, and what best-effort delivery means
title: Outbound Mail
type: guide
---

Rela can send email. This guide covers configuring a mail transport and what to
expect from it.

> **Scheduled mail is recipient-scoped.** There is no broadcast mode: every
> message is rendered through its recipient's ACL visibility. See
> [Scheduled declarative mail](#scheduled-declarative-mail).
>
> **Read the delivery guarantee section before you rely on this.** Delivery is
> best-effort by design, and the failure mode is specific enough to be worth
> understanding up front.

## Quick Start

Create `.rela/mail.yaml` in your project:

```yaml
transport: smtp
host: smtp.example.com
port: 587
username: relay@example.com
from: rela@example.com
from_name: "Rela"
base_url: https://rela.example.com
```

Then put the password in `.rela/secrets.yaml`:

```yaml
smtp_password: your-smtp-password
```

If `.rela/mail.yaml` does not exist, mail is simply off. Every other command works
exactly as before — an absent config is a normal state, not an error.

## Configuration

| Key | Required | Description |
| --- | --- | --- |
| `transport` | yes | `smtp`, `memory`, `http` or `script` |
| `host` | for smtp | SMTP server hostname. A bare hostname, not a URL |
| `port` | no | Defaults to `587` |
| `username` | no | Omit for a relay that accepts unauthenticated submission |
| `password_env` | no | **Name of an environment variable** holding the credential. Optional — see below |
| `account_id` | for http | Provider account the API endpoint is scoped to |
| `script` | for script | Project-relative path to a `.lua` send script |
| `capabilities` | for script | What the send script may reach — see below |
| `from` | yes | Envelope and header sender |
| `from_name` | no | Display name for the sender |
| `timeout_seconds` | no | Per-send timeout. Defaults to `30` |
| `base_url` | no | Public app URL, used to resolve links in mail |

### Choosing a transport

`smtp` covers the common deployment and is what you want unless you have a
reason not to. The other three exist for specific situations:

- **`memory`** records messages in process instead of sending them. For local
  development and tests — see [Local development](#local-development).
- **`http`** delivers over the SimpleMailService APIv2 HTTP API. Useful where
  outbound port 587 is blocked, or where you already have an API token.
- **`script`** runs a Lua script you supply. This is the answer for every
  provider rela does not ship: you write the mapping from a rendered message
  onto your provider's request, and no rela release is involved when that
  provider changes its API.

There is deliberately **no field-mapping DSL**. Provider send APIs disagree on
encoding, auth scheme, sender shape, recipient shape and body field names —
Mailgun is not even JSON, it is `multipart/form-data` with HTTP Basic — so any
mapping layer general enough to be worth learning would still have excluded
providers by construction. A script is both smaller and complete.

### The password lives with your other secrets

Put it in `.rela/secrets.yaml`, alongside every other credential rela uses:

```yaml
# .rela/secrets.yaml
smtp_password: your-smtp-password     # transport: smtp
mail_api_token: your-api-token        # transport: http
jira_api_key: sk-abc123
```

That is the same file Lua scripts read. An SMTP password is no different in kind
from an API token, so it goes in the same place rather than in a mechanism unique
to mail.

`transport: script` is the exception: its credential is whatever key your script
names, and you grant it explicitly under `capabilities.secrets`. See
[The script transport](#the-script-transport).

**Or use an environment variable.** If your deployment injects credentials as
environment variables — containers, systemd units — name the variable instead:

```yaml
# .rela/mail.yaml
password_env: RELA_SMTP_PASSWORD
```

`secrets.yaml` takes precedence when both are present.

Either way, **the password never goes in `mail.yaml`**. Writing a literal
`password:` key there is refused at load with an error pointing you at the
alternatives. This is the rule that keeps the database DSN env-only too: a
secret must never reach a command line, where it lands in `ps` output and shell
history. `.rela/` is gitignored by convention, but a config file is still
something people paste into bug reports.

### TLS is mandatory

Connections use STARTTLS, and rela **refuses to send** to a server that does not offer
it rather than falling back to an unencrypted connection. Certificates are verified;
there is deliberately no option to skip verification.

If you are running a mail server that only speaks plaintext, put a TLS-terminating
relay in front of it rather than asking rela to downgrade.

## The http transport

```yaml
# .rela/mail.yaml
transport: http
account_id: your-account-id
from: notifications@example.com
from_name: Example
```

```yaml
# .rela/secrets.yaml
mail_api_token: your-api-token
```

The endpoint is `https://api.simplemailservice.eu/v2` and is **not
configurable**. An operator-settable endpoint would turn this into a
"POST my mail anywhere" primitive carrying a live credential, which is a
redirect away from being a credential-exfiltration hole. If you need a
different provider, that is what `transport: script` is for.

Inline images ride as base64 attachments whose `file_name` matches the `cid:`
reference in the HTML.

## The script transport

```yaml
# .rela/mail.yaml
transport: script
script: mail/mailgun.lua
from: notifications@example.com
from_name: Example
capabilities:
  http: true
  secrets: [mailgun_key, mailgun_domain]
```

`script` is a **project-relative path to a `.lua` file inside the project**.
Absolute paths and paths escaping the project are refused at load: a send script
runs with outbound HTTP and a credential, so where it comes from is worth being
strict about.

### What the script gets

A global `message` table:

| Field | Type | Notes |
| --- | --- | --- |
| `to` | array of `{email, name}` | At least one recipient |
| `subject` | string | |
| `html` | string | May be empty if the message is text-only |
| `text` | string | May be empty if the message is HTML-only |
| `from` | `{email, name}` | From `mail.yaml`, not from the script |
| `rendered_for` | string | Whose visibility bounded the content |
| `inline_images` | array of `{cid, content_type, data}` | Absent when there are none |

Plus `http.*` (including `form` for multipart and `basic_auth`), `crypto.*`
(including `base64_encode`/`base64_decode`), `rela.json.*`, and the secrets you
granted as `rela.secrets.<key>`.

### What it does not get

**No graph access at all.** The runtime is built with no read or write
dependencies, so `rela.get_entity`, `rela.list_entities`, `rela.search`, every
traversal binding and every mutation binding are unavailable. A send script
receives an already-rendered message and can only ship it. Content was
ACL-scoped upstream when it was rendered, so there is nothing a send script
needs the graph for — and this way the restriction holds by construction rather
than by a rule someone has to remember.

Secrets are narrowed the same way: `capabilities.secrets` is a **list of key
names**, never a boolean. A key you do not list is *absent* from
`rela.secrets`, not empty — so a typo surfaces as a nil at the use site instead
of authenticating with `""`.

### Reporting failure

Raise an error (or call `error(...)`) and the outbox treats it as a failed send,
retrying through the normal backoff ladder. **Do not swallow a non-2xx status:**
returning normally tells rela the mail was delivered.

### Where credentials come from

The secrets scope is the **configured script path**, so ordinary per-script
overrides work with no mail-specific convention:

```yaml
# .rela/secrets.yaml
mailgun_key: key-shared
overrides:
  mail/mailgun.lua:
    mailgun_key: key-just-for-mail
```

The path is the scope key rather than a triggering user because the outbox
delivers minutes after the fact, on a background worker, on a retry — there is
no triggering user in scope, and the credential belongs to you rather than to
whoever happened to save an entity. Deliveries are audited as `system:mail`.
`message.rendered_for` still names the identity whose visibility bounded the
*content*, which is a different question and the one that matters for ACL.

### Example scripts

`examples/mail/` ships working scripts for Mailgun, Postmark and Resend. Copy
one into your project and adjust it.

They are **examples, not supported integrations.** They target third-party APIs
rela does not control and cannot version. rela's tests pin them against local
stubs, which proves rela's side of the contract and proves nothing about whether
the provider still accepts those field names today. When a provider changes its
API, edit your copy — that is the point of shipping the mapping as Lua.

### Sending mail from any script

Beyond the transport, `mail.send` is available to Lua scripts generally:

```lua
local ok, err = mail.send{
  to = "alice@example.com",
  subject = "Report ready",
  html = "<p>Done.</p>",
  text = "Done.",
}
if not ok then
  print("mail failed: " .. err.kind .. " " .. err.message)
end
```

It delivers through whichever transport the project configured, so a script
cannot reach a destination you did not set up. The binding is **always present**,
even with no `mail.yaml` — when mail is off it returns
`err.kind == "not_configured"` rather than vanishing, so a script can
feature-detect. A delivery failure returns `(nil, err)` and never raises: a
script that mails a summary at the end of a run should not lose the run because
the mail server was rebooting.

## Delivery is best-effort

Mail is queued in memory and delivered by a background worker. This keeps sends off
the request path: a save is never blocked waiting on a mail server, and a mail server
that is down degrades to a logged delivery failure instead of a failed write.

The cost is that **undelivered mail does not survive a restart**, and the details
matter:

- In **`rela-server`** there is no shutdown handler, so there is no drain at all —
  every message still queued when the process stops is lost.
- Where shutdown does run — CLI commands, the desktop app, tenant eviction — the
  worker gets a bounded window (10s) to finish in-flight sends before it is
  abandoned.
- A failed send is retried with exponential backoff, up to 5 attempts, and then
  given up on with an error in the log.
- **One bad message delays the good ones.** Delivery is sequential, so an
  undeliverable message — a typo'd address, most likely — holds up everything
  behind it for the length of its retry ladder, about 30 seconds. A batch of bad
  addresses adds up.
- If the queue fills (128 messages), further sends are **rejected with an error**
  rather than silently dropped. A full queue means the mail server is unreachable
  and a backlog is building — or that a stalled message is holding the line.
- If a shutdown drain times out (10s), the send in flight is left to finish on its
  own and its connection is not returned. On a server hosting many projects, where
  eviction happens routinely, those connections accumulate against your provider.

So: **mail here is notification, never a system of record.** Do not build anything
that assumes a message arrived. If you need delivery guarantees, send to a real queue
or a transactional mail provider that provides them.

A durable queue with swappable backends is planned; when it lands, this caveat
narrows considerably.

## Scheduled declarative mail

Put project-owned message declarations in `mail-templates.yaml`:

```yaml
mail_templates:
  overdue_digest:
    subject: "Tasks due {{today}}"
    intro: "Items visible to you that need attention."
    address_property: email
    sections:
      - title: "Overdue"
        entity_type: task
        where: ["status != done", "due < today"]
        columns: [title, due, owner]
```

```yaml
# schedules.yaml
tasks:
  - name: daily-digest
    template: overdue_digest
    every: day
    for_each:
      entity_type: person
      where: ["active = true"]
```

There is deliberately no broadcast mode. The scheduler posts one child job per
selected recipient. Each child resolves the current address and renders through
that recipient's row- and field-visible reader before sending. One failed
recipient retries independently.

Sections support `style: table` (the default), `list`, and `detail`. `rela validate`
checks template keys, schema references, filters, scheduled template
references, and the recipient address property. Runtime validates the current
address again.

Pending task + occurrence + recipient identities suppress concurrent duplicate
work. A retry after a completed child can send again, and SMTP has its own
acknowledgement crash window, so delivery is at-least-once rather than exactly-once.

Automation-triggered templates are planned separately.

**Sending from Lua** will add a `mail.send` binding for anything the declarative
form does not cover.

For local verification, point SMTP at a capture tool such as
[Mailpit](https://mailpit.axllent.org/) or use the memory transport below.

## Local development

Use the `memory` transport to exercise mail without a mail server — and without any
risk of mailing real people:

```yaml
transport: memory
from: rela@example.com
```

Messages are recorded in process instead of being sent. This is the recommended
setting for `just dev` and for any test environment pointed at production-like data.

## Branding

Mail reuses your existing theme rather than introducing a second one. The accent
colour comes from your palette, and the operator logo is embedded in the message.

Two constraints worth knowing:

- **The logo is embedded, not linked.** The logo endpoint sits behind authentication,
  so a mail client could never fetch it — the bytes travel with the message.
- **SVG logos are skipped.** SVG has near-zero support in mail clients and is an
  active-content format. If your logo is an SVG, mail renders without it; upload a
  PNG if you want it to appear.

Messages are sent as `multipart/alternative` — an HTML part plus a plain-text
alternative — using a table-based layout that renders correctly in Outlook, Gmail and
Apple Mail.

## Multi-tenant deployments

Each project gets its own mail worker and its own connection to your mail server. A
server hosting many projects against one mail provider will open one connection per
project; check that against your provider's connection limits.

## Troubleshooting

**Mail is off and I don't know why.** An absent `.rela/mail.yaml` is silent by
design. An *invalid* one logs a warning at startup naming the problem — check the
server log for `mail: disabled`.

**Sends fail with a TLS error.** The server must offer STARTTLS with a valid
certificate. A self-signed certificate will be rejected; use a real one, or terminate
TLS at a relay.

**`transport: http` says "no API token".** Set `mail_api_token` in
`.rela/secrets.yaml`, or name an environment variable in `password_env`.

**A send script fails with "attempt to call a nil value" on `http`.** You did not
grant the capability. Add `capabilities: {http: true}` to `mail.yaml`.

**A send script reads `nil` for a secret it expects.** The key is not in
`capabilities.secrets`. That list is exact — a key you did not name is absent,
which is what makes a typo visible instead of silent.

**Nothing arrives and there are no errors.** Check the log for `mail: delivery
failed`. If the process restarted while messages were queued, they are gone — see
the delivery section above.
