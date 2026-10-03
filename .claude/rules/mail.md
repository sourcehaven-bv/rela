---
paths:
  - "internal/mail/**"
  - "internal/mailrender/**"
  - "internal/lua/mail*.go"
---

# Mail

- **Mail: the render pipeline order is a security property, and delivery is
  best-effort.** `internal/mailrender` runs markdown → goldmark → **bluemonday
  on the untrusted CONTENT ONLY** → trusted template → **douceur inline LAST**.
  Both ends are load-bearing and verified: bluemonday strips `style` attributes
  (so sanitizing the assembled document ships unstyled mail, and also strips the
  `cellpadding`/`border`/`role` and `cid:` sources email needs), while douceur
  does **no** CSS value validation (so nothing may sanitize after it, and every
  value interpolated into CSS — palette tokens included — must be allowlisted).
  Reversing either is a silent downgrade, not a build failure.

  **`lua` may import `mailrender`, and that does NOT invert the `mail → lua`
  arrow.** `mail.render` (TKT-1GA2PG) builds a `mailrender.Message` from a
  script table so a Lua author gets the hardened template instead of
  hand-writing HTML for `mail.send`. The arch-lint rule forbidding `lua → mail`
  is untouched and still holds; `mailrender` is a _different_ component and a
  true leaf (`go list -deps` shows zero internal imports), so the two arrows
  cannot form a cycle. The binding must never grow an `html:` or `css:` field —
  that would reintroduce the sanitizer bypass it exists to give authors an
  alternative to.

  **Email CSS is not web CSS, and the template's shape encodes that.** Section
  headings and the empty-section note are single-cell tables, and vertical gaps
  are spacer rows, because Outlook Windows honors `padding` only on table cells
  and `margin` is unsupported or partial across Gmail, Outlook, Yahoo and AOL. A
  `<div>` with padding renders fine wherever you are likely to test it and
  collapses where you are not. `internal/mailrender/compat_test.go` scores the
  rendered output against a **vendored, pinned** Can I Email dataset
  (`testdata/caniemail.min.json`) and fails on a regression — treat it as a
  floor, not proof the mail looks right.

  **Dark mode is defensive, and `<meta name="color-scheme">` is deliberately
  absent.** Clients split three ways: some leave mail alone (Apple Mail, Gmail
  desktop, Yahoo, AOL), some partially invert and honor `prefers-color-scheme`
  (the Outlook family), and some fully invert and rewrite the query to
  `@media none` so they cannot be targeted at all (Gmail iOS/Android, Outlook
  Windows). The `@media` block serves the middle group; the palette (mid-tone
  borders, no pure white on pure black) serves the third. Adding the meta tag
  looks like a free win and is the trap: it opts Apple Mail _into_ inverting,
  making a currently-correct rendering worse. A test asserts its absence.

  **A message's language belongs on `Message`, never on `Options`.** `Options`
  is renderer-scoped branding and a `Renderer` is built once per deployment, so
  an `Options.Lang` would stamp one language on every mail an instance sends —
  and a Dutch digest and an English one cannot both be right. `Options` carries
  only the _default_. The tag is validated in `mailrender` (shape-only BCP-47,
  rejected not escaped) because it arrives from both operator config and
  untrusted Lua, and validating at either call site would leave the other open.

  The SMTP password lives in **`.rela/secrets.yaml`** under `smtp_password` —
  the same store Lua scripts read, because an SMTP credential is no different in
  kind from the API tokens already kept there. `password_env` in
  `.rela/mail.yaml` names an environment variable as a fallback for
  container/systemd deployments; secrets.yaml wins when both are set. Never a
  literal `password:` in mail.yaml — that is refused at load.

  Header-injection validation is **rela's**, not the SMTP library's: go-mail
  rejects CR/LF in addresses but accepts it in a subject, where it is
  neutralized only incidentally by encoded-word escaping. `internal/mail`
  rejects CR/LF/NUL in every caller-supplied header value at enqueue.

  The outbox is an in-process buffer, **not a durable queue** — in `rela-server`
  there is no signal handler, so pending mail is lost on every restart with no
  drain. Mail is notification, never a system of record. A durable queue with
  swappable backends is IDEA-WIJ2H1.
