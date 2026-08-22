---
id: PLAN-EQC0Q8
type: planning-checklist
title: 'Planning: Mail foundation: Sender interface (SMTP + in-memory), best-effort outbox, branded HTML rendering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IS: `internal/mailrender` (leaf: message model → branded HTML + text/plain bytes);
`internal/mail` (Sender interface, SMTP transport via go-mail, memory transport,
`.rela/mail.yaml` config, best-effort in-process outbox + worker); wiring in
`appbuild.assemble`; docs.

IS NOT: HTTP-API transport, Lua script transport, `mail.send` binding (all
TKT-DS1CR6); `mail_templates:` config, scheduler/automation triggers, recipient
resolution from the graph, per-recipient ACL scoping (all TKT-U2R7GU); durable
queue (IDEA-WIJ2H1); inbound mail, bounce handling, tracking; DKIM (the provider
signs).

Nothing in this ticket sends mail on its own — there is no trigger yet. The
deliverable is a Sender that a later ticket calls. Verification is therefore via
tests plus a **build-tag-gated** manual trigger (`//go:build mailmanual`, absent from
the default build and asserted so in CI), NOT via a shipped user-facing feature and
NOT via "temporary code removed before merge".

**Acceptance Criteria:** the 12 on the ticket. Each is mapped to a concrete test in
the Test Plan below.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the option space was settled in the ticket discussion
(transport survey across 4 providers, MJML-vs-Go-templates, best-effort-vs-durable).
Findings recorded on TKT-332QZY / TKT-DS1CR6 / IDEA-WIJ2H1 rather than a separate
RES entity.

*Correction from design review (N1).* The first pass of this checklist marked
research done while **assuming** two library behaviours that a fifteen-line probe
disproved: bluemonday strips `style` (RR-7BYA4X) and douceur validates nothing
(RR-S0I1WH). Both invalidated the core pipeline. Every library claim below is now
from executed code at the pinned version, not from documentation. The lesson worth
carrying: "reviewed the docs" is not research when the design hinges on what a
library actually *does* to a specific input.

**Existing Solutions:**

*SMTP library — `github.com/wneessen/go-mail` v0.8.1 (CHOSEN).*
`net/smtp` is **frozen** (its own package doc: "not accepting new features") and
provides only the wire protocol — message assembly would be ours: RFC 5322 header
folding, RFC 2047 encoded-words for non-ASCII subjects, quoted-printable, multipart
boundaries. All are easy to get subtly wrong in ways that surface in one client.
go-mail is MIT, actively maintained, and **verified**: it requires only
`golang.org/x/crypto`, `golang.org/x/text` and `golang.org/x/net`, all three already
in this repo's graph at *newer* versions (v0.55.0 / v0.41.0 / v0.58.0 vs its
v0.54.0 / v0.40.0 / v0.56.0). So it adds **one module and forces no version bump**
— confirm with `go mod graph` in the PR rather than restating this from the plan.
Rejected: `emersion/go-message`+`go-smtp` (same maintainer as the in-tree go-ical /
go-webdav, but two modules and more glue for no gain).

*CSS inlining — `github.com/aymerick/douceur/inliner` (CHOSEN).*
Already in go.sum at v0.2.0 (indirect, via glamour → bluemonday → douceur/parser),
so no version negotiation. **Verified working** against the exact use case:
`inliner.Inline(html)` turns a `<style>` block into inline `style=""` attributes and
strips the `<style>`, which is precisely the Outlook/Gmail requirement.
Cost correction: `douceur/inliner` (unlike `douceur/parser`, the part already
linked) pulls `PuerkitoBio/goquery` + `andybalholm/cascadia` — **2 new modules**.
Rejected: `vanng822/go-premailer` — not in go.sum at all, and depends on douceur +
goquery anyway, so strictly more cost for the same result.

*HTML sanitizing — `bluemonday` v1.0.27*, in go.sum (indirect). Promoting it to
direct is a mechanical `go mod tidy` move, no new module.

*Markdown — `goldmark` v1.8.5*, already a direct dep (FEAT-010).

*MJML — REJECTED.* It is a build-time compiler emitting static table HTML, not a
runtime engine. Running it needs Node, which rela (static Go binary) has no
guarantee of; `cmdexec` fails closed, so mail would silently break per host. The
plan copies MJML's *compiled output* into Go templates once and pins with goldens.

*Codebase patterns reused (all verified at the cited lines):*
- `internal/calfeed` — the leaf-package contract `mailrender` copies (pure
  model→bytes, no store/metamodel imports). `.go-arch-lint.yml:59` declares it with
  **no `deps:` entry at all**, which is how a true leaf is expressed.
- `internal/ai/config.go:26-79` — `LoadConfig` shape: `ConfigFile` const,
  `ErrConfigNotFound` sentinel for absent-file, `os.ReadFile` →
  `errors.Is(os.ErrNotExist)` → `yaml.Unmarshal` → `cfg.Validate()`, every error
  wrapping `path`. `Validate` (config.go:83-121) rejects credentials-in-URL and
  query strings; `api_key_env` names an env var read **at call time**
  (openai.go:370-380) so unrelated commands start with it unset.
- `internal/ai/redact.go:11-24` — `redactKey` scrubbing at error/log construction.
- `internal/store/pgstore/sweep.go:121-160` — the worker lifecycle to copy verbatim:
  detached `context.WithCancel(context.Background())`, `done chan struct{}`,
  `defer close(done)` as the first statement of `run`, nil-safe idempotent `stop()`,
  and `err != nil && ctx.Err() == nil` before logging (suppresses the spurious
  cancel-during-work error — essential for a worker mid-send at shutdown).
- `internal/appbuild/datamigration.go:16-80` — `startX(...) (stop func())` returning
  a no-op `func(){}` on every not-configured branch, never an error; torn down in
  `Services.Close` (appbuild.go:1356-1376).
- `internal/scheduler/scheduler.go:79-90` — `WorkspaceProvider`, a **structural**
  consumer-side interface. `appbuild` does not import `scheduler` (verified: no
  match), yet `*Services` satisfies it. This is the pattern that avoids the plimsoll
  problem below.
- `internal/store/storetest/storetest.go:150` + `internal/state/statetest` — the
  conformance-suite shapes for the transport suite.
- `internal/mcp/golden_test.go:51-79` — golden files gated on `UPDATE_GOLDEN=1`
  (env var, not a flag), with nondeterminism normalized **at capture time**
  (regex-replacing minted IDs) rather than by weakening the comparison.
- `internal/store/pgstore/listener_test.go` — `goleak` (already a direct dep) for a
  goroutine-owning package.
- `internal/lua/http.go:36-79` — bounded `http.Transport`. Not needed this ticket
  (no HTTP transport until TKT-DS1CR6) but noted so `mail` does not invent its own.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Two packages, split by dependency weight.*

`internal/mailrender` — a genuine leaf. Input is a plain struct (subject, intro
markdown, sections, a `map[string]string` of colour tokens, optional logo bytes);
output is `(htmlBytes, textBytes, error)`. It imports goldmark, bluemonday, douceur
and **nothing from `internal/`**. Deliberately takes `map[string]string` rather than
`dataentryconfig.ResolvedPalette`, because `dataentryconfig` transitively depends on
`filter`/`git`/`metamodel` — taking the map keeps the leaf genuinely dependency-free
and satisfies AC 12 with the fewest new arch-lint edges.

*Pipeline — the ordering is LOAD-BEARING, not incidental (RR-7BYA4X).*

```
untrusted entity markdown
  → goldmark
  → bluemonday.Sanitize   ← STEP 1: the CONTENT ONLY, never the whole page
  → embed in trusted Go template (600px tables, <!--[if mso]> fallbacks)
  → douceur/inliner        ← STEP 3: LAST. Nothing may strip style after this.
  → HTML part
```

Verified empirically against bluemonday v1.0.27 and douceur v0.2.0 (not assumed):

- **bluemonday's `UGCPolicy` strips `style` attributes outright**, and
  `AllowStyling()` does **not** restore them. So sanitizing *after* inlining would
  destroy every inlined style and ship unstyled mail.
- Sanitizing the *assembled page* would also strip `cellpadding` / `cellspacing` /
  `border` / `role` from our own trusted template — the exact attributes email
  clients need — and would drop `cid:` image sources, breaking the embedded logo.
- Confirmed the safe order works end to end: `<script>`, `onerror=`,
  `javascript:` hrefs and `style="background:url(javascript:…)"` are all removed
  from content, while template styles inline correctly and `<!--[if mso]>`
  conditional comments survive the inliner intact.

Therefore: **sanitize untrusted content, then template, then inline. Never sanitize
the assembled page.** State this in the package doc — reversing it is a silent
downgrade (mail still sends, just unstyled or unsafe), so a comment is the only
thing standing between a future edit and the bug.

**Which policy.** `bluemonday.UGCPolicy()` + `AllowTables()`, applied to the content
fragment. Naming the policy is the security decision in `mailrender`; leaving it
unstated was a gap (the generic "sanitize" tests pass under any policy).

*The corollary the ordering creates — douceur is an unvalidated attribute injector
(RR-S0I1WH).* Verified: `inliner.Inline` does **zero** CSS value validation. It
materializes `background:url('javascript:alert(1)')`, `behavior:url(x.htc)` (IE HTC
script execution) and `width:expression(alert(1))` straight into `style=` attributes
— *after* the only sanitizer has run. Two consequences:

1. **Only the trusted template's `<style>` block may feed the inliner.** Untrusted
   content is sanitized to a fragment beforehand and never contributes CSS. This is
   what makes the ordering safe rather than merely convenient.
2. **Palette tokens are an injection channel and must be allowlisted.** The plan
   passes `map[string]string` colour tokens into CSS; nothing made them colours.
   Verified an accent value of `url('javascript:alert(1)')` lands verbatim in a
   `style` attribute. So: validate each token against `^#[0-9a-fA-F]{3,8}$` (or a
   named-colour set) at the `mailrender` boundary and **reject** — do not escape,
   do not substitute a default silently.

The `text/plain` part is generated from the same model, not from the HTML.
`html.WithUnsafe()` must NOT be used: unlike the existing `simpleMarkdownToHTML`
(dataentry/helpers.go:324-331) whose input is operator-authored schema prose, mail
carries **entity content**, which is untrusted.

`internal/mail` — Sender + config + outbox.

```go
type Sender interface { Send(ctx context.Context, m Message) error }
```

`Message` carries already-rendered parts (the renderer runs upstream), so a transport
never re-renders and can never disagree with another transport about output.

**But that makes mail an ACL exfiltration surface with no enforcement point
(RR-S4I9M9).** Once bytes reach `Send`, nothing distinguishes "rendered from
ACL-gated reads" from "rendered from a raw store handle" — and mail crosses the
perimeter *irrevocably*; you cannot un-send. Deferring per-recipient scoping to
TKT-U2R7GU is a fine scope call; deferring the **seam** is not, because the follow-up
would then bolt a gate onto the call site — exactly the per-consumer redaction
CLAUDE.md forbids.

So `Message` carries the **principal it was rendered for** as a required field, set
by the caller. This ticket does not *enforce* anything with it (there is no trigger
yet), but it is the named anchor TKT-U2R7GU attaches its `internal/visibility` gate
to, and the package doc says so. Ship the hole knowingly, with the door frame already
in the wall.

- `smtpSender` wraps go-mail. STARTTLS **required**: set
  `WithTLSPolicy(mail.TLSMandatory)` **explicitly** rather than relying on go-mail's
  default, which a dependency bump could change; `TLSOpportunistic` falls back to
  unencrypted port 25 and must never be used, nor the `quicksend` helper that
  selects it. Certificate verification stays on — if AC 2's local fake needs
  `InsecureSkipVerify`, STARTTLS-required becomes theatre, so the fake must present
  a real cert (test CA) and the config must not expose a skip-verify knob.
- `memorySender` records into a bounded ring buffer; exposes a read accessor for
  tests and dev. Two genuine uses (local dev without a mail server; assertions on
  what *would* have been sent) — and it makes the interface exercised rather than
  asserted from day one.
- Config `.rela/mail.yaml` mirroring `ai/config.go` exactly, with `password_env:`.
  `Validate()` rejects a literal password field outright (so the mistake is loud, not
  a silently-leaking config).
- Outbox: in-process channel + bounded buffer, worker goroutine, capped exponential
  backoff. Enqueue never dials and returns `ErrOutboxFull` at capacity (RR-RSRUXK)
  rather than dropping silently — a full buffer means the mail server is down and
  the backlog is building, which is an operational condition, not a normal one.
  **Best-effort by design** — stated in the package doc and operator docs, not
  merely implied.
- **No idempotency key.** With one sequential worker and no persistence, dedup is
  guaranteed by construction; a key would defend against nothing and read as cargo
  cult. It belongs with the durable backend (IDEA-WIJ2H1), where concurrent
  consumers make it load-bearing. AC 9 is satisfied by the retry loop not
  re-enqueueing, and its test asserts exactly that.

*Wiring — and the constraint that shapes it.*
`appbuild.Services` is at **exactly 25/25** exported methods against
`//plimsoll:max-exported-methods=25` (appbuild.go:86 — verified by count). Adding a
`Services.Mail()` accessor would fail the god-object lint and require arguing the cap
upward. Avoided entirely via a narrow **structural** provider interface (the
`scheduler.WorkspaceProvider` precedent), so nothing is added to `Services`' public
surface and no `appbuild → mail` accessor edge appears.

**The interface, written down (RR-CVTBSK)** — the whole mitigation rests on this, so
it is specified now rather than discovered at implementation:

```go
// in internal/mail — consumer-side, satisfied structurally by *appbuild.Services.
type Provider interface {
    Paths() *project.Context   // appbuild.go:130 — exists
    State() state.KV           // appbuild.go:252 — exists
}
```

Verified both accessors already exist, so **zero new exported methods on `Services`**.
Everything else the renderer needs — the resolved palette map and the logo bytes — is
**passed as plain values at the wiring site**, not fetched through `Services`. That
matches the existing `app.SetCalDAVAliases(svc.CalDAVAliases())` hand-off and keeps
`mailrender` free of a `dataentryconfig` dependency.

Lifecycle: the worker starts in `assemble` via `startMailWorker(...) (stop func())`
following `startDataMigration`, with a `mailStop` field torn down in `Services.Close`
before the store closes. `stop()` takes a **bounded drain timeout** (RR-RSRUXK) —
without one, a worker mid-send against an unresponsive server blocks `Close`
indefinitely.

**Multi-tenancy (RR-X06ZWR).** `Assemble` runs once per store, so this is **one
worker per tenant** — matching the per-assembled `startDataMigration` GC sweep. Each
worker sends sequentially (no fan-out), so per-tenant concurrency is 1, but a
deployment with N tenants against one provider gets N connections; say so in the docs.
A *shared* sender is explicitly rejected here: it could not live in `assemble` without
violating the `SharedBase` rule that `Close` tears down only per-assembled resources.
Revisit under IDEA-WIJ2H1, where a shared queue backend changes the picture.

**Constructors reject nil (house rule).** `mailrender.New`, the SMTP sender
constructor and the outbox constructor all return `error` on nil required
collaborators. Note the distinction from `startMailWorker` returning a no-op `stop`
when mail is unconfigured: that is "feature off", which is legitimate. Silently
substituting a no-op *Sender* would not be — it defers the failure to a downstream
symptom, which the house rules forbid.

*Branding — a constraint found during research.*
The operator logo is stored in `state.KV` under `theme/logo` + `theme/logo.ext`
(dataentry/theme_logo.go:13-14, cap 256 KiB) and served from
`/api/v1/_theme/logo?v=<hash>` — which **is** an `isAPIPath`, hence behind the auth
gate. **An email client cannot fetch it.** So the logo must be embedded as a
**CID attachment** from the stored bytes (go-mail supports inline embeds), not
linked.

**Raster only — SVG is refused (RR-PIBARP).** `allowedLogoExts`
(theme_logo.go:24-28) permits SVG, but SVG in email has near-zero client support
(Gmail/Outlook/Apple Mail strip or fail it) *and* is an active-content format that
can carry `<script>`. CID-embedding operator-uploaded SVG would ship script-capable
bytes into inboxes. So the mail logo takes an explicit `png/jpeg/webp` allowlist;
an SVG logo is skipped and the mail renders without one. "Known ext, unsuitable for
email" is a distinct case from "unknown ext" and needs its own branch. Colours come from `dataentryconfig.ResolvePalette(...).Light`, whose
`--accent-color` default is `#4772fb` (palette.go:164), flattened to a
`map[string]string` at the wiring site. This reuses the existing theme mechanism
rather than inventing a second one, per the ticket.

**Files to modify:**

New:
- `internal/mailrender/{mailrender.go,template.go,text.go,mailrender_test.go}`
- `internal/mailrender/testdata/*.golden.html`
- `internal/mail/{mail.go,config.go,smtp.go,memory.go,outbox.go,redact.go}` + tests
- `internal/mail/mailtest/mailtest.go` — transport conformance suite
- `internal/appbuild/mail.go` — `startMailWorker`

Modified:
- `go.mod` / `go.sum` — +go-mail; promote bluemonday, douceur to direct
- `internal/appbuild/appbuild.go` — `mailStop` field, `assemble` call, `Close` teardown
- `.go-arch-lint.yml` — declare `mail`, `mailrender`; register `gomail`,
  `bluemonday`, `douceur` **plus `goquery` and `cascadia`** (douceur/inliner's deps)
  as vendors; `mail.mayDependOn: [mailrender]`; add `internal/mail/mailtest` to the
  top-level `exclude:` list alongside `storetest`/`statetest` (verified at
  `.go-arch-lint.yml:8-16` — test kits are excluded individually, so a new one is
  flagged unless listed)
- `.testcoverage.yml` — floors for the two new packages
- `docs/` — a mail page; the best-effort caveat stated explicitly

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `.rela/mail.yaml` | operator, local disk (gitignored) | `Validate()`: host non-empty, port in range, transport ∈ {smtp, memory} (**allowlist**), reject a literal `password:` key, reject credentials in any URL form | load error naming the file, mail off |
| SMTP password | env var named by `password_env` | read at **call time**, never at load | send fails with a typed error; startup unaffected |
| Entity content → mail body | the graph (untrusted) | goldmark → **bluemonday** sanitize; never `WithUnsafe()` | script/handler stripped |
| Recipient addresses | caller (TKT-U2R7GU) | parse/validate before enqueue; reject CR/LF (**header injection**) | rejected at enqueue |
| Subject / headers | caller | go-mail owns encoded-words; reject embedded CR/LF | rejected at enqueue |
| Logo bytes | `state.KV`, operator-uploaded | already capped at 256 KiB with an ext allowlist upstream | omitted from mail |

**Security-Sensitive Operations:**
- **Credential handling** — password never in YAML, never on a command line (the
  `RELA_DATABASE_URL` invariant), never in logs or errors. A `redactKey` equivalent
  is applied at every error-construction site. AC 11 tests this.
  Note `rela validate` prints validator errors verbatim (cli/validate.go:410), so a
  mail validator must never embed the env **value** in its message.
- **Header injection** — the classic SMTP hole, and **our** job, not the library's
  (RR-CC6VEW). Verified against go-mail v0.8.1: `From()`/`To()` *do* reject embedded
  CRLF with a parse error, but `Subject("Hi\r\nBcc: evil@x.com")` **succeeds** and is
  neutralized only incidentally, by RFC 2047 encoded-word escaping (`=0D=0A`). That
  is an encoding side effect, not a validation guarantee — it is not safe to depend
  on across versions, nor for header values the library does not encode.
  So: CR/LF (and bare CR, bare LF, NUL) is **rejected at enqueue** on every
  caller-supplied header value, uniformly. go-mail's address checking is defence in
  depth, not the control.
- **TLS** — STARTTLS required; downgrade refused.
- **HTML sanitization** — mail is an *exfiltration and injection* surface: content
  leaves the ACL perimeter into an inbox. Sanitizing is not optional.
- **Resource bounds** — outbox buffer bounded (drop-with-log at capacity rather than
  unbounded growth); message size capped; logo capped upstream.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|---|---|
| 1 config absent → off | `LoadConfig` on an empty dir returns `ErrConfigNotFound`; `startMailWorker` returns a no-op stop; a CLI command runs normally |
| 2 SMTP + STARTTLS | in-process SMTP fake (net.Listener speaking enough of the protocol); assert AUTH after STARTTLS; a **separate** case asserts a server offering no STARTTLS is refused |
| 3 memory records | send → assert recipients/subject/both parts; assert no dial occurred |
| 4 conformance | `mailtest.RunAll(t, factory)` — both transports registered from external test packages (`mail_test`), storetest style |
| 5 render golden | markdown with h1/h2, link, bold, GFM table → `.golden.html`; `UPDATE_GOLDEN=1` to regenerate; boundary/Message-ID/Date normalized at capture |
| 6 sanitize | `<script>alert(1)</script>`, `<img onerror=…>`, `<a href="javascript:…">` and `style="background:url(javascript:…)"` in content → all absent from HTML part |
| 6a ordering (RR-7BYA4X) | the rendered HTML **has** inline `style=` attributes — a regression that sanitizes the assembled page would strip them, so this test catches the ordering inversion that AC 6 alone would pass |
| 6b mso survives | `<!--[if mso]>` conditional comments present in the final output |
| 6c CSS injection (RR-S0I1WH) | **post-inline** assertion that no `style=` attribute in the output contains `javascript:`, `behavior:` or `expression(` — AC 6 cannot detect this, since every policy passes the `<script>`/`onerror` cases |
| 6d palette allowlist | a palette token of `url('javascript:alert(1)')` is **rejected** at the `mailrender` boundary, not escaped or defaulted |
| 7 text alternative | every rendered message has non-empty text/plain; a table degrades to readable text |
| 8 enqueue non-blocking | enqueue against a transport that blocks forever returns immediately (with timeout) |
| 9 retry no-dup | transport fails twice then succeeds → exactly one delivery recorded; assert backoff grew |
| 10 loss on restart | enqueue, stop worker with items pending, assert undelivered — **pins best-effort as intended behaviour** |
| 11 no credential leak | set password env to a sentinel; drive config-load, send-failure and validate paths; assert the sentinel appears in no log/error string |
| 12 arch-lint | `just arch-lint` in CI |

**Edge Cases:**
- Empty body, empty subject, zero recipients (reject at enqueue).
- Non-ASCII subject (encoded-words) and non-ASCII body (quoted-printable) — the
  reason go-mail was chosen; assert a UTF-8 subject round-trips.
- Very long single-line content (header folding).
- Markdown with no headers/links (degenerate but valid).
- Content that is *only* an HTML block.
- Outbox at capacity — bounded, drop-with-log, asserted.
- Worker stopped mid-send — `stop()` returns, no goroutine leak (`goleak`).
- Concurrent enqueue from many goroutines under `-race`.
- Logo absent / oversized / unknown ext → render without it, never fail the send.
- Logo is **SVG** (a *known* but unsuitable ext, RR-PIBARP) → skipped, not embedded.
- Outbox full → `ErrOutboxFull` returned to the caller (RR-RSRUXK), not a silent drop.
- `stop()` while a send is blocked on an unresponsive server → returns within the
  bounded drain timeout, abandoning in-flight work rather than hanging `Close`.

**Negative Tests:**
- `transport: carrier-pigeon` → load error naming valid values.
- `password:` literal in YAML → load error telling the operator to use `password_env`.
- `password_env` naming an unset variable → send fails with a typed error; **startup
  still succeeds** (the ai precedent).
- CR/LF injected into a **recipient** → rejected at enqueue.
- CR/LF injected into a **subject** → rejected at enqueue (RR-CC6VEW). Must assert
  rejection *at the boundary*, not merely that the emitted header looks escaped —
  the latter passes today by accident of encoded-word escaping and would keep
  passing if the validation were deleted.
- Bare CR, bare LF, and NUL in any header value → rejected.
- SMTP server rejecting AUTH → typed auth error, no credential in the message.
- Golden mismatch → fails with the "review the diff, never regenerate to make it
  green" guidance.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| `Sender` shaped only by in-process transports; TKT-DS1CR6 adds remote ones | `Message` carries pre-rendered parts and `Send` takes a ctx — both remote-friendly. Re-read TKT-DS1CR6's sketch before finalising the signature. |
| **plimsoll cap**: `Services` at 25/25 exported methods | Structural provider interface in `internal/mail`; nothing added to `Services`' public surface. Verified as the pattern `scheduler` already uses. |
| douceur/inliner adds goquery + cascadia (2 modules) | Accepted: the alternative is hand-written CSS inlining, which is worse. Both are widely used and already in the wider Go ecosystem. Flag in PR. |
| Mail lost on restart | **Intended**; documented and pinned by AC 10. Retired by IDEA-WIJ2H1. |
| Rendering regressions across clients | Goldens pin *our* output. They cannot prove real-client rendering — full client-matrix testing is explicitly out of scope; say so rather than implying coverage. |
| Nothing triggers mail in this ticket | Verification is tests plus a **`//go:build mailmanual` gated** trigger — NOT "temporary code removed before merge", which is how debug endpoints reach production. CI asserts the default build does not include it. |
| No graceful shutdown in `rela-server` (`main.go:531`, no signal handler) | Out of scope. `Services.Close()` does run for CLI/desktop/tenant-eviction, so the drain path is real there; note the server gap in docs rather than fixing it here. |

**Effort:** M (unchanged).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/` — a new mail page: `.rela/mail.yaml` reference, `password_env`, STARTTLS
      requirement, branding, and the **best-effort delivery caveat stated plainly**.
      Not the euphemism "a notification can be lost" but the specific truth
      (RR-RSRUXK): *`rela-server` has no signal handler, so `Services.Close` never
      runs there — every pending message is lost on every restart, with no drain at
      all. The drain path is real only for CLI, desktop and tenant eviction.* Also
      note N tenants means N provider connections (RR-X06ZWR).
      Source-edited under `docs-project/` since `docs/*.md` are generated.
- [x] `CLAUDE.md` — a short entry: mail config lives in `.rela/`, the renderer is a
      leaf, the outbox is best-effort, and where the durable-queue plan lives.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change this ticket)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no new command)
- [x] ~~`docs/data-entry.md`~~ (N/A: no UI change)
- [x] ~~`README.md`~~ (N/A: not project-level until the declarative layer ships)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 8 findings, all addressed in this plan. Every library
claim was verified by executing the pinned version rather than reading docs.

| ID | Sev | Finding |
|---|---|---|
| RR-7BYA4X | critical | bluemonday strips `style`; sanitize-after-inline would ship unstyled mail, and sanitizing the assembled page also strips `cellpadding`/`border`/`role` and `cid:` sources. Ordering is load-bearing and now stated as such. |
| RR-S0I1WH | critical | douceur does **zero** CSS validation and runs last — `javascript:`/`behavior:`/`expression()` reach `style` attrs after the sanitizer. Palette tokens are the live channel; now allowlisted and rejected, with a post-inline AC. |
| RR-CC6VEW | significant | go-mail rejects CRLF in addresses but **not** in `Subject` (only incidental encoded-word escaping). Header-injection validation is ours, at enqueue, for every header value. |
| RR-S4I9M9 | significant | Pre-rendered `Message` left no anchor for TKT-U2R7GU's ACL gate. `Message` now carries the principal it was rendered for. |
| RR-CVTBSK | significant | The plimsoll mitigation rested on an unwritten interface. Now specified; verified `Paths()`/`State()` already exist, so zero new exported methods. |
| RR-PIBARP | significant | SVG logos are script-capable and unsupported in mail; CID embedding restricted to raster. |
| RR-X06ZWR | significant | Multi-tenancy unaddressed: one worker/connection per tenant. Decision stated and documented. |
| RR-RSRUXK | significant | Stale `net/smtp` in the ticket (fixed); no drain in `rela-server` (documented plainly); silent drop-on-full → `ErrOutboxFull` + bounded drain timeout. |

Minor items folded in without separate entities: explicit `TLSMandatory` (not the
default) and no skip-verify; `goquery`/`cascadia` vendor registration plus the
`mailtest` arch-lint exclusion; idempotency key **dropped** as unmotivated for a
single sequential worker; the manual trigger is build-tag gated rather than
"removed before merge"; go-mail's dependency cost restated precisely.
