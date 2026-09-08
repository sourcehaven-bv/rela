---
id: TKT-1GA2PG
type: ticket
title: Mail template compatibility hardening + a mailrender-backed render path for Lua
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

Two halves of one problem, which is why they are one ticket.

The mail that prompted this (Mailpit `5TvadkwnFjwGC9xxIj6O2T`, "MT-agenda
2026-09-02") is **not** rendered by `internal/mailrender`. It is hand-built HTML
from `atlas/scripts/mt-agenda-mail.lua`, passed straight to
`mail.send{html=...}`, which forwards the string untouched.
`internal/mailrender` is reachable only from `appbuild.RunScheduledTemplate`;
nothing on the Lua path can reach it.

That is the second half of this ticket: a script author who wants branded,
client-compatible mail today has no option but to hand-write HTML and get it
wrong. Fixing rela's template alone would not improve that mail by one pixel.

The first half is the template itself. It was audited by rendering a
representative message through the real `mailrender.Renderer` (subject, intro, a
linked table, a markdown body section, an empty section, a footer, a CID logo)
and scoring every CSS property and HTML element in the output against the **Can
I Email** dataset (`caniemail` npm package: 307 tracked features across ~43
client/platform combinations).

**The template holds up well.** CSS inlining works, the `<!--[if mso]>` wrapper
survives douceur, `bgcolor` fallbacks are emitted alongside `background-color`,
and there are no catastrophic failures. The findings are real but incremental.

## Part A — template findings (evidence-backed)

### A1. `padding` on `<div>` — silently dropped in several clients

Can I Email `css-padding` note #1: *"Partial. Only supported on table cells."*

The rendered output puts `padding` on four `<div>`s: `.sect-title` × 3
(`padding: 20px 0 8px 0`) and `.empty` × 1 (`padding: 8px 0`).

In Outlook Windows / Windows Mail this padding is dropped, so section headings
lose their vertical rhythm and collide with the table above. Highest-value fix.

### A2. `margin` on `<table class="tbl">`

`css-margin` is partial in 16 client/platform combos including all of Gmail,
Outlook Windows, Yahoo and AOL. `.tbl` sets `margin: 0 0 12px 0`, which is the
only thing separating a table from the next section heading. Where margin is
dropped, tables butt directly against the following heading. (`.sect`'s `margin:
0` is harmless.)

### A3. `role="presentation"` on the *data* table — accessibility defect

`.tbl` is a genuine data table with `<th>` headers, but it carries
`role="presentation"`, which tells a screen reader to ignore the header
association entirely. That role is correct for the two **layout** tables
(`.wrap`, `.card`) and wrong here. The `<th>` also lack `scope`.

Plain WCAG rather than a Can I Email finding, but it surfaced while reading the
output and it is a one-line fix.

### A4. No `lang` attribute on `<html>` — and it must be PER-MESSAGE

Affects screen-reader pronunciation and some clients' translation prompts. The
Atlas mail is Dutch; nothing in the document says so.

**Language is a property of the message, not of the deployment.** One rela
instance can send a Dutch MT-agenda and an English digest from the same
renderer, so a renderer-scoped `Options.Lang` would be wrong by construction:
`Options` carries BRANDING (palette, logo, BaseURL), which is deployment-wide,
and a `Renderer` is built once per send in `appbuild.RunScheduledTemplate` — one
`Options.Lang` would label every mail the instance ever sends with one language.

`lang` therefore belongs on **`Message`**, beside `Subject` — both are content,
not branding. It is surfaced at all three call sites:

- `lang:` per template under `mail_templates:` — `mailtemplate.Template` already
mirrors `Message` field-for-field (`subject`/`intro`/`sections`), so this is
where the field naturally lands.
- `lang = "nl"` in the Lua `mail.render` table (Part B).
- `Message.Lang` for direct Go callers.

An operator-level default covers the common case where a deployment's mail is
all one language; a per-message value overrides it. Empty means "fall back to
the default", never "emit an empty attribute".

### A5. Dark mode — DEFENSIVE only, and the `color-scheme` meta tag is a TRAP

The naive framing ("rela has no dark mode, add one") is wrong, and so was this
ticket's first draft. **You do not get to opt out of dark mode.** Roughly a
third of opens are in a dark-mode client (Litmus, *The Ultimate Guide to Dark
Mode for Email Marketers*), and clients that support it will invert the mail
*for* you. The only choice is whether the inversion is yours or theirs.

Three tiers, confirmed against the Can I Email dataset
(`css-at-media-prefers-color-scheme`, last tested 2023-03-08):

| Tier | Clients | Behaviour |
|---|---|---|
| **No change** | Apple Mail, Gmail desktop, Yahoo, AOL | renders the light design as authored |
| **Partial invert** | Outlook.com, Outlook iOS/Android/macOS | light backgrounds darken, dark ones don't; `prefers-color-scheme` **works** |
| **Full invert** | Gmail iOS/Android, Outlook Windows, Windows Mail | everything inverts; `prefers-color-scheme` **does not work** |

The dataset makes the third tier explicit: the query is rewritten to `@media
none` (#2), `@media (false)` (#5), `@media ()` (#6) or `@media ( _filtered_a )`
(#1) depending on client. It works in **18/43** client/platform combos.

**The correction to this ticket's first draft:** it proposed adding `<meta
name="color-scheme" content="light dark">` as a harmless extra for the 2 clients
that honour it. Litmus names that exact thing as a trap — *"Apple Mail applies
partial invert if Dark Mode meta tags are included **without** styles."* Apple
Mail currently sits in the leave-it-alone tier, so adding the tag **opts Apple
Mail into inverting** rela's mail. Without a complete matching dark stylesheet
that is a regression in the one major client that renders the template exactly
as designed. **The meta tag is not added.**

So the scope here is deliberately small — three cheap, defensive changes, not a
dark theme:

1. **No `color-scheme` meta tag** (avoids the Apple Mail trap; it buys 2/41
clients and costs the best-behaved one).
2. **A `@media (prefers-color-scheme: dark)` block** that explicitly sets
card/bg/text/border, so the partial-invert tier gets a coherent result rather
than a half-inverted one. This is the tier where the query verifiably works.
3. **Palette hardening for the clients that cannot be targeted at all.** This is
what helps the full-invert tier. Per Litmus: avoid pure `#ffffff` on near-black,
prefer midtones, and don't depend on a border colour that vanishes under
inversion — rela's `#e5e7eb` border on a `#ffffff` card is exactly that case.

**Not doing** the `data-ogsc`/`data-ogsb` attribute-prefixed duplication Litmus
also recommends: it is Outlook.com-specific, roughly doubles the stylesheet, and
would have to survive douceur. Not worth it at rela's volume — recorded here so
the omission is a decision rather than an oversight.

**Evidence caveat:** the dataset row was last tested 2023-03-08 and the Litmus
article is older. The tiers are broadly stable, but exact per-client claims
should be treated as approximate, and none of this substitutes for looking at
the result in a real client.

**Constraint from CLAUDE.md:** douceur inlines the `<style>` block and a
`@media` rule cannot be inlined — it must survive as a `<style>` block in
`<head>`. **Verified by experiment:** `inliner.Inline` leaves `@media` blocks
intact in `<head>` while inlining the inlinable rules. So this is feasible.

### A6. Minor / accepted as-is

- `border-radius` fails in 7 clients (incl. Outlook Windows) — degrades to
square corners. No action.
- `max-height` on the logo fails in Outlook Windows — a large logo could render
oversized. Addressed by adding explicit `width`/`height` attributes on `<img>`.
- `height` on `.bar` is buggy in 6 clients (replaced by `min-height`) — the
`height="4"` presentation attribute is already there as fallback. No action.
- `text-decoration` fails on 3 iOS clients — links still render as links. No action.

## Part B — a mailrender-backed render path for Lua

A script should be able to hand rela **markdown plus structure** and get back
the same branded, compatibility-hardened HTML the declarative scheduled-mail
path produces.

### The dependency direction is the whole design

`internal/mail` already depends on `internal/lua` (for `transport: script`), and
`.go-arch-lint.yml` states the rule explicitly: *"The arrow points mail -> lua
and NEVER back."* So `lua` must not import `mail`.

But `internal/mailrender` is a **true leaf** — its arch-lint entry allows only
goldmark/bluemonday/douceur and no internal components at all. Therefore `lua ->
mailrender` introduces **no cycle** and is the clean seam. This mirrors how
`mail` itself depends on mailrender "for the body model only".

Two viable shapes; the planning phase picks one:

- **B-i (preferred): a render binding.** `mail.render{...} -> html, text`
builds a `mailrender.Message` from a Lua table and returns both parts, which the
script then passes to the existing `mail.send`. Composable, keeps `mail.send` a
thin pass, and makes the rendered output inspectable/testable from Lua.
- **B-ii: a structured `mail.send`.** `mail.send` grows optional
`intro`/`sections`/`footer` fields, mutually exclusive with `html`. Fewer calls
for the common case, but overloads one function with two very different
contracts and hides the rendered output.

### Trust and safety

Script-supplied markdown is **untrusted**, exactly like entity content: it goes
through the same goldmark → bluemonday → template → douceur pipeline, in that
order. The script gets **no** control over the `<style>` block, the palette, or
raw HTML passthrough — otherwise the binding becomes a way to smuggle
unsanitized markup past the sanitizer, which is precisely what
`mail.send{html=...}` already allows and what this path exists to give people an
alternative to.

`safeHref`'s scheme allowlist applies unchanged to script-supplied row links.

## Scope: IS

**Part A**

1. Move `padding` off `<div>` onto table cells for `.sect-title` and `.empty`,
or restructure them into single-cell tables (the MJML-compiled-output approach
the template already follows).
2. Replace `.tbl`'s bottom `margin` with spacing that survives — a spacer row or
cell padding.
3. Drop `role="presentation"` from `.tbl`; add `scope="col"` to its `<th>`.
4. Add `lang` to `<html>` as a **per-message** value: `Message.Lang`, a `lang:`
key on `mailtemplate.Template`, and a `lang` field in the Lua `mail.render`
table, over an operator-level default.
5. Add `width`/`height` attributes to the logo `<img>`.
6. Dark mode, defensively: a `prefers-color-scheme: dark` block for the
partial-invert tier, palette hardening for the full-invert tier, and **no**
`color-scheme` meta tag.
7. **A repeatable compatibility check**: a Go test that renders the golden
sample, extracts CSS properties and elements, and asserts none appear on an
element type the dataset marks unsupported. The dataset is vendored as a pinned
JSON fixture, never fetched at test time.

**Part B**

8. A `lua -> mailrender` binding (shape chosen in planning) that turns a Lua
table into a `mailrender.Message` and renders it with the default template.
9. Register `mailrender` under the `lua` component in `.go-arch-lint.yml`, with
a comment recording why the direction is safe (mailrender is a leaf; the `mail
-> lua` arrow is untouched).
10. Table-driven tests for the Lua binding: shape/type errors raise (argument
errors), markdown is sanitized, `<script>`/`javascript:` hrefs are stripped, row
links honour `safeHref`, and the output carries inline `style=` attributes.
11. Docs for the new binding alongside the existing `mail.send` docs.

## Scope: IS NOT

- Fixing `atlas/scripts/mt-agenda-mail.lua` — different repository. This ticket
makes the good path *available*; migrating Atlas onto it is Atlas's change.
- Removing or deprecating `mail.send{html=...}`. Raw HTML stays supported; the
`transport: script` path in `internal/mail` depends on pre-rendered bodies.
- A live client-rendering matrix (Litmus / Email on Acid) — paid SaaS that
cannot run in CI. The vendored dataset check is the offline substitute.
- MJML at runtime. Rejected in TKT-332QZY and still rejected: it reintroduces a
Node toolchain dependency.
- Changing the render pipeline order, which is a security property.
- Giving Lua control over the palette, the `<style>` block, or the CID logo.
- **Translating content.** `lang` LABELS the language of content the caller
already supplies; rela does not translate, and there is no per-locale string
catalogue in this ticket. The one template-authored English string ("Nothing to
show.") is noted as a known wart, not fixed here.
- **A full dark theme, the `color-scheme` meta tag, and `data-ogsc`/`data-ogsb`
duplication** — see A5. Dark handling here is defensive only.
- **Dark-mode logo swapping.** If an operator's logo is a transparent PNG with
dark artwork it goes invisible in dark mode; rela cannot know that from the
bytes, and a light/dark image pair is a bigger feature.

## Acceptance criteria

**Part A**

1. No `padding` declaration appears on a non-`<td>`/`<th>` element in rendered output.
2. No `margin` declaration is the sole provider of spacing between a table and
the following section heading.
3. `.tbl` has no `role="presentation"` and its `<th>` carry `scope="col"`; the
two layout tables keep `role="presentation"`.
4. `<html>` carries a `lang` attribute driven **per message**: two messages
rendered by the SAME `Renderer` with different `Message.Lang` values emit
different `lang` attributes. A `lang:` on a mail template reaches the output; a
`lang` in the Lua `mail.render` table reaches the output; an unset value falls
back to the operator default rather than emitting `lang=""`.
5. The logo `<img>` carries `width` and `height` attributes.
6. A `@media (prefers-color-scheme: dark)` block survives douceur into the final
output. The output contains **no** `<meta name="color-scheme">` — asserted
explicitly, with the Apple Mail rationale in the test name, so a future
"improvement" that adds it fails.
7. A compatibility test fails when a future edit puts `padding` back on a `<div>`.
8. Golden files updated; existing security assertions (mso comments survive,
`style=` attributes present, no `javascript:`/`behavior:`/`expression(`
post-inline) still pass unchanged.

**Part B**

9. A Lua script can build a message from markdown + sections and obtain HTML
that is byte-identical to what `mailrender` produces for the same model — pinned
by a test that renders the same model through both paths and compares.
10. Script-supplied markdown containing `<script>`, `onerror=` and a
`javascript:` href is stripped from the rendered HTML.
11. A row link with a `javascript:` or relative scheme is dropped (text still
renders, unlinked), matching `safeHref`.
12. The rendered HTML carries inline `style=` attributes — the ordering-inversion
canary from TKT-332QZY criterion 6a, re-asserted on this path.
13. A malformed call (wrong types, missing subject) **raises**; it does not
return an error table. Delivery failures keep returning error tables.
14. `just arch-lint` passes with `mailrender` registered under `lua`, and the
`mail -> lua` direction is unchanged.

**Both**

15. `just test`, `just lint`, `just coverage-check` pass.

## Risks

- **Restructuring section headings into tables changes the golden files
substantially**, making the security assertions harder to eyeball in review.
Mitigation: keep those assertions as explicit content assertions rather than
relying on golden-file diffing.
- **The Lua binding becomes a sanitizer bypass** if it ever grows a raw-HTML or
raw-CSS field. Mitigation: no such field in this ticket, and the package doc
records why.
- **`lua -> mailrender` looks like it inverts the `mail -> lua` arrow** to a
future reader. Mitigation: the arch-lint comment states that mailrender is a
leaf and the two arrows are independent.
- **`lang` is an attribute value, so it must be validated**, not just escaped —
a conservative BCP-47 shape allowlist (letters/digits/hyphen, bounded length),
rejected rather than sanitized. It arrives from operator config AND from
untrusted Lua, so validation happens in `mailrender` where both paths converge,
not at either call site.
- **Someone re-adds the `color-scheme` meta tag** believing it is a free win.
Mitigation: AC6 asserts its absence and the template comment records that it
opts Apple Mail into inversion.
- **Dark-mode overrides are unverifiable in CI.** The `@media` block's *presence*
is testable; whether it *looks right* is not. Mitigation: keep the override set
minimal (bg/card/text/border) so the blast radius is small, and treat visual
confirmation in a real client as follow-up rather than claiming it here.
- **The vendored Can I Email dataset goes stale** (its dark-mode row was last
tested 2023-03-08). Mitigation: it is a compatibility floor, not a source of
truth; the fixture records its version and update date.
