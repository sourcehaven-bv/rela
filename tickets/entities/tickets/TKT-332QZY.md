---
id: TKT-332QZY
type: ticket
title: 'Mail foundation: Sender interface (SMTP + in-memory), best-effort outbox, branded HTML rendering'
kind: enhancement
priority: medium
effort: m
status: in-progress
---

## Description

First of three. Ships the send path end-to-end with SMTP, plus an in-memory
sender for local dev and tests. No declarative config (TKT-U2R7GU) and no Lua
(TKT-DS1CR6).

`internal/mail` (Sender interface, SMTP + memory transports, config, best-effort
outbox) and `internal/mailrender` (markdown → sanitized, CSS-inlined, branded
table HTML + text/plain).

## Scope: IS

**`internal/mail` — transports + config + outbox**

```go
type Sender interface {
    Send(ctx context.Context, m Message) error
}
```

- **`transport: smtp`** — via `github.com/wneessen/go-mail` (NOT `net/smtp`, which is
frozen and would leave RFC 5322 message assembly to us). STARTTLS mandatory,
explicit timeouts. Must be Go; the Lua sandbox has no socket access by design.
Verified against `send.simplemailservice.eu:587`, but nothing provider-specific
is compiled in — it is ordinary authenticated SMTP.
- **`transport: memory`** — records messages in a bounded ring buffer instead of
sending. Two real uses, not a test double bolted on:
  - **Local dev**: `just dev` exercises digests and notifications with no mail server
and no risk of mailing real people. Sent messages are inspectable in-process.
  - **Tests**: e2e and integration tests assert on what *would* have been sent
(recipients, subject, rendered parts) without an SMTP fake in every test.

It also keeps the `Sender` interface honest from day one: with two
implementations the abstraction is exercised rather than asserted. (Same posture
as `memstore`/`LinearSearch` — a real backend, selected by config, that happens
to be ideal for testing.)
- **Config** `.rela/mail.yaml`, modeled on `internal/ai/config.go`: `ErrConfigNotFound`
sentinel (absent file = feature off, not an error), a `Validate()` rejecting
credentials-in-URL, and a `redactKey` safety net. Credential is `password_env:`
— an env var **name** resolved at call time, never a literal and never a flag
(the `RELA_DATABASE_URL` invariant: a secret must never reach a command line).

**Outbox + worker — deliberately BEST-EFFORT.**

- Enqueue appends to an in-process buffer; a worker goroutine delivers with capped
exponential backoff. Writes never block on SMTP; a down mail server degrades
rather than failing saves.
- Enqueue returns `ErrOutboxFull` at capacity rather than dropping silently — a
full buffer means the mail server is down and the backlog is building.
- **No idempotency key**: one sequential worker with no persistence makes dedup
true by construction. It belongs with the durable backend (IDEA-WIJ2H1).
- **This is a delivery buffer, not a durable queue.** A crash or restart with
undelivered mail loses it. Accepted for this ticket, not an oversight — a real
queue seam with swappable backends (memory/postgres/redis) is IDEA-WIJ2H1, and
this outbox becomes its first consumer when it lands.

State it plainly in the package doc and operator docs: **a notification can be
silently lost on restart.** Mail is notification, never a system of record;
nothing may be built on an assumed delivery guarantee. Be specific in the docs:
in `rela-server` there is no signal handler, so `Services.Close` never runs and
**every pending message is lost on restart with no drain at all** — the drain
path is real only for CLI, desktop and tenant eviction.
- Consequently **no `state.KV` persistence here.** Keep the buffer behind a narrow
internal interface (enqueue / claim / mark-done) so a queue backend can be
substituted without touching mail code.

**`internal/mailrender` — a leaf package**, mirroring `internal/calfeed`'s
contract: pure model → bytes, no store/metamodel imports (arch-lint forbids
`transform` importing `dataentry`, and the existing goldmark converter is
unexported in `dataentry` and uses `WithUnsafe()`).

- goldmark → **bluemonday sanitize (the untrusted CONTENT ONLY)** → embed in the
trusted template → **douceur inline LAST**. The order is load-bearing, verified:
bluemonday strips `style` attributes, so sanitizing the assembled page ships
unstyled mail and also strips `cellpadding`/`border`/`role` and `cid:` sources.
And douceur does zero CSS validation, so nothing may sanitize after it. Entity
content is untrusted; `WithUnsafe()` must NOT be reused here.
- Palette colour tokens are validated against an allowlist and **rejected** if not
colours — they flow into CSS, and douceur would materialize
`url('javascript:…')` into a `style` attribute verbatim.
- Table-based 600px layout with `<!--[if mso]>` fallbacks, CSS inlined via douceur
(already in go.sum). This is MJML's *compiled output* hand-ported once into Go
templates — same client compatibility, zero runtime dependency, cannot fail for
a missing Node toolchain.
- `multipart/alternative` with a generated `text/plain` alternative (deliverability).
- **Operator branding**: accent colour + logo, reusing the existing operator
logo/theme rather than a second theme mechanism. The logo is **CID-embedded**
(it sits behind the auth gate, so a mail client cannot fetch it) and **raster
only** — SVG is script-capable and unsupported in mail, so it is skipped.
- Golden-file tests pin the HTML so layout regressions surface in review.

## Scope: IS NOT

- No HTTP-API transport and no Lua script transport — TKT-DS1CR6. `Sender` must not
acquire anything that presumes an in-process transport, or that ticket has to
reshape it.
- No `mail_templates:` config, scheduler/automation triggers, recipient resolution, or
per-recipient ACL scoping — TKT-U2R7GU.
- No durable queue (IDEA-WIJ2H1).
- No inbound mail, bounce handling, or open/click tracking.
- No DKIM signing in rela (the transport provider signs).

## Acceptance criteria

1. `.rela/mail.yaml` absent → mail is off and every other command starts normally.
2. `transport: smtp` delivers to a local SMTP fake, STARTTLS negotiated; a plaintext
downgrade is refused.
3. `transport: memory` records the message and does not dial; the recorded message
exposes recipients, subject, and both rendered parts.
4. Both transports satisfy one shared conformance suite (the storetest pattern).
5. Markdown with headers, links, bold and a table renders to sanitized inlined-CSS
HTML; golden-file pinned.
6. `<script>`, `onerror=`, `javascript:` hrefs and `style="…url(javascript:…)"` in
entity content are stripped from the HTML part. 6a. The rendered HTML **has**
inline `style=` attributes — catches an ordering inversion that criterion 6
alone would pass. 6b. `<!--[if mso]>` conditional comments survive to the final
output. 6c. **Post-inline**, no `style=` attribute contains `javascript:`,
`behavior:` or `expression(`. 6d. A palette token of
`url('javascript:alert(1)')` is rejected at the renderer boundary, not escaped
or silently defaulted.
7. Every message carries a non-empty `text/plain` alternative.
8. Enqueue returns without dialing; delivery happens on the worker.
9. A transport failure retries with backoff and does not duplicate the message.
9a. CR/LF in a **subject** is rejected at enqueue (go-mail does not reject it;
only incidental encoded-word escaping neutralizes it today). 9b. `stop()`
returns within a bounded drain timeout even when a send is blocked against an
unresponsive server.
10. Restart with a non-empty buffer loses the mail **and says so** — an explicit test
documenting best-effort, so the limit is pinned rather than discovered.
11. Credential never appears in logs, errors, or `/api/v1/_config`.
12. `just arch-lint` passes with the new components registered.

## Risks

- **Blocking the write path** — mitigated by making enqueue the only synchronous step.
- **Mail lost on restart** — accepted and documented; retired by IDEA-WIJ2H1.
- **`Sender` shaped around SMTP** — the memory impl gives a second shape immediately,
but neither is remote-API-shaped. Sanity-check the interface against the
HTTP/script transport sketch in TKT-DS1CR6 before finalising, so that ticket is
not forced to reshape it.
- **Rendering regressions** across mail clients — mitigated by golden files; a full
client matrix is out of scope.
