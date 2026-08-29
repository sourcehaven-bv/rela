# Example mail send scripts

These scripts back `transport: script` in `.rela/mail.yaml`. Each takes an
already-rendered message and ships it to one provider.

**They are examples, not supported integrations.** They target third-party
APIs that rela does not control and cannot version. rela's tests pin them
against local stubs, which proves *our* contract — the `message` global, the
`http`/`crypto` primitives, the error convention — and proves nothing about
whether Mailgun still accepts these field names today. When a provider changes
its API, the fix is to edit your copy of the script; no rela release is
involved, which is the entire point of shipping the mapping as Lua rather than
compiling it in.

## Using one

Copy the script into your project (anywhere under the project root; `mail/` is
conventional), then point `.rela/mail.yaml` at it:

```yaml
transport: script
script: mail/mailgun.lua
from: notifications@example.com
from_name: Example
capabilities:
  http: true
  secrets: [mailgun_key]     # a LIST of key names, never a bool
```

Put the credential in `.rela/secrets.yaml`:

```yaml
mailgun_key: key-...
mailgun_domain: mg.example.com
```

Per-script credentials work exactly as they do for any other script — the
secrets scope is the configured script path:

```yaml
overrides:
  mail/mailgun.lua:
    mailgun_key: key-a-different-one
```

## What a send script gets

- `message` — a global table: `to` (array of `{email, name}`), `subject`,
  `html`, `text`, `from` (`{email, name}` from mail.yaml), `rendered_for`, and
  `inline_images` (array of `{cid, content_type, data}`) when there are any.
- `http.*` — including `form = {...}` for multipart/form-data and
  `basic_auth = {user =, pass =}`.
- `crypto.*` — including `base64_encode` / `base64_decode`.
- `rela.secrets` — only the keys `capabilities.secrets` names.

## What it does NOT get

No graph access at all. The runtime is built with no read or write
dependencies, so `rela.get_entity`, `rela.list_entities`, `rela.search` and
every mutation binding are unavailable — a send script receives a rendered
message and can only ship it. Content was ACL-scoped upstream when it was
rendered, so there is nothing here a script needs the graph for.

## Failing

Raise an error (or call `error(...)`) to report a delivery failure. The outbox
treats it as any other failed send: it retries through the existing backoff
ladder and logs it after the last attempt. Do not swallow a non-2xx status —
returning normally tells rela the mail was delivered.
