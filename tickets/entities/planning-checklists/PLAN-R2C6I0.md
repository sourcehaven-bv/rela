---
id: PLAN-R2C6I0
type: planning-checklist
title: 'Planning: Mail template compatibility hardening + a mailrender-backed render path for Lua'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Two halves, deliberately in one ticket because half A without half B does not
improve any mail anyone is actually sending today.

**A — harden `internal/mailrender`'s template** against real client-support
data. The findings came from rendering a representative message through the
actual `mailrender.Renderer` and scoring every CSS property and HTML element in
the output against the Can I Email dataset (`caniemail` npm, 307 features, ~43
client/platform combos). See the ticket for the per-finding evidence.

**B — give Lua a `mailrender`-backed path**, so a script can hand rela markdown
plus structure and get the same branded, hardened HTML back instead of
hand-writing `<div>` soup.

IS NOT: fixing `atlas/scripts/mt-agenda-mail.lua` (different repo — this ticket
makes the good path available, migrating Atlas is Atlas's change); removing
`mail.send{html=...}` (the `transport: script` path depends on pre-rendered
bodies); Litmus/Email-on-Acid (paid SaaS, cannot run in CI); MJML at runtime
(rejected in TKT-332QZY, reintroduces a Node dependency); changing the render
pipeline order (a security property); giving Lua control of the palette,
`<style>` block, or CID logo; **translating content** (`lang` labels, it does
not translate).

**Acceptance Criteria:** 15 criteria on the ticket. Each maps to a test scenario
in the Test Plan below.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the research was done inline and is recorded here and on
the ticket. It is empirical (dataset queries + two executed experiments) rather
than a survey of competing designs, so a RES entity would duplicate rather than
add.

**Existing Solutions:**

*Compatibility-checking tools surveyed:*

| Tool | Verdict |
|---|---|
| **Can I Email** (`caniemail` npm) | **Chosen.** MIT, offline JSON, 307 features × ~43 client/platform combos, machine-readable with per-note caveats. Vendorable as a fixture. |
| Litmus / Email on Acid | Rejected. Real screenshots across clients, but paid SaaS requiring an account and network — cannot run in CI. |
| MJML | Rejected again (TKT-332QZY already rejected it). Node toolchain at runtime. Note the current template is a hand-port of *MJML's compiled output*, which is why it scores as well as it does. |
| `html-validate` / vnu | Rejected as the primary check. They validate HTML correctness, not mail-client support — they would pass a `<div>` layout that Outlook mangles. |
| bluemonday / douceur | Already in use, correctly ordered. No change. |

*Prior art in the codebase:*

- `internal/mailrender/template.go:20-27` — the `msoOpen`/`msoClose` constants
and the comment explaining that `html/template` strips HTML comments at parse
time. Any new conditional-comment content must follow the same
pass-as-`template.HTML` trick.
- `internal/lua/mailrecipients.go:89-95` — `RecipientPolicyCarrier`, an
**optional interface** a `MailSender` may implement, chosen over widening
`MailSender`. This is the exact pattern Part B needs for `BaseURL` (see
Approach).
- `internal/lua/mail.go:81-95` — `registerMailModule` uses a **method value**,
not a closure, to avoid a `contextcheck` false positive. The new binding must
register the same way.
- `internal/mailtemplate/mailtemplate.go:29-32` — `Template` mirrors
`mailrender.Message` field-for-field (`subject`/`intro`/`sections`). That
mirroring is what makes `lang` land naturally as a per-template key.
- `internal/mailrender/mailrender_test.go` — 22 existing tests including
`TestRender_KeepsInlineStyles` (the ordering-inversion canary),
`TestRender_PreservesMSOConditionals`, and
`TestRender_NoDangerousCSSReachesStyleAttributes`. These are the assertions that
must keep passing unchanged.
- `internal/appbuild/scheduled_mail.go:64` — the only current `mailrender`
construction site; the model it builds is the reference shape for Part B.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Three facts established by inspection/experiment, not assumption

**1. douceur PRESERVES `@media` at-rules.** This was the open question blocking
the dark-mode item. Ran `inliner.Inline` over a stylesheet containing `@media
(prefers-color-scheme: dark)` and `@media only screen and (max-width:600px)`:
douceur inlined the inlinable rules onto elements *and* left both media blocks
intact in a `<style>` block in `<head>`. So A5 is feasible, and a mobile
breakpoint becomes possible as a bonus (not claimed in this ticket's scope).

**2. `lua -> mailrender` is provably cycle-free.** `go list -deps
internal/mailrender | grep Sourcehaven` returns **only mailrender itself** —
zero internal dependencies, a true leaf. `go list -deps internal/mail | grep
internal/lua` **does** match, confirming `mail -> lua` exists (the `transport:
script` arrow that `.go-arch-lint.yml` says must never reverse). Since
mailrender imports nothing internal, adding `lua -> mailrender` cannot close any
cycle. The two arrows are independent.

**3. `lang` cannot live on `Options`.** `Options` is renderer-scoped and carries
BRANDING only — palette, LogoCID, LogoAlt, BaseURL — all of which are
deployment-wide. `appbuild.RunScheduledTemplate:64` constructs a `Renderer` per
send from the deployment's mail config, so an `Options.Lang` would stamp one
language on every mail the instance ever sends. **Language is content**, and it
belongs on `Message` beside `Subject`. Confirmed by inspecting both structs
(`mailrender.go:95-110` for `Message`, `:114-134` for `Options`).

### Part A — template changes (`internal/mailrender/template.go`)

- **A1 `padding` on `<div>`:** convert `.sect-title` and `.empty` from `<div>`
to single-cell tables (`<table><tr><td class="sect-title">`). This is what MJML
compiles to and what the rest of the template already does, so it is consistent
rather than a special case.
- **A2 `.tbl` bottom margin:** replace `margin: 0 0 12px 0` with a trailing
spacer row (`<tr><td height="12"
style="font-size:0;line-height:0;">&nbsp;</td></tr>`), mirroring how `.bar`
already uses a `height` attribute plus zeroed font-size/line-height.
- **A3 roles:** drop `role="presentation"` from `.tbl` only; add `scope="col"`
to its `<th>`. `.wrap` and `.card` keep theirs — they are layout tables.
- **A4 `lang` — PER-MESSAGE.** Three coordinated changes:
  - `mailrender.Message.Lang string` — the value for this message.
  - `mailrender.Options.DefaultLang string` — the deployment fallback,
defaulting to `"en"`, used when `Message.Lang` is empty. This is the only part
that is renderer-scoped, and it is a *default*, not the value.
  - Resolution in `buildDocument`: `lang := m.Lang; if lang == "" { lang = r.opts.DefaultLang }`.
  - Emitted as `<html lang="{{.Lang}}" xmlns=...>`.

Surfaced at the three call sites: `Lang` on `mailtemplate.Template` (yaml
`lang:`), a `lang` key in the Lua `mail.render` table, and `Message.Lang` for Go
callers.

**Validation lives in `mailrender`**, where the operator path and the
untrusted-Lua path converge — validating at either call site would leave the
other open. A conservative BCP-47 *shape* allowlist
(`^[A-Za-z]{1,8}(-[A-Za-z0-9]{1,8})*$`, bounded total length), rejected rather
than sanitized. Deliberately a shape check, not a registry lookup: rela has no
business refusing a valid-but-unusual subtag, and the security requirement is
only that nothing can break out of the attribute.
- **A5 dark mode:** add a `@media (prefers-color-scheme: dark)` block overriding
card/bg/text/border with `!important`. Values come from the same validated
palette map — add dark tokens with defaults, validated by the existing
`ValidatePalette`. Add `<meta name="color-scheme" content="light dark">` for the
2 clients that honour it.
- **A6 logo:** add `width`/`height` attributes to the `<img>`, from new
`Options.LogoWidth`/`LogoHeight` (ints, omitted when zero). These ARE branding,
so `Options` is right for them.

### Part B — the Lua binding

**Shape B-i chosen: `mail.render{...} -> html, text`.** Rejecting B-ii
(overloading `mail.send`) because `mail.send`'s package doc is explicit that it
"does not render, does not template" — folding rendering in would contradict a
documented contract and give one function two incompatible argument shapes. B-i
keeps `mail.send` a thin pass, makes the rendered output inspectable from Lua
(so a script can log or diff it), and composes:

```lua
local html, text = mail.render{
  subject = "Wekelijks MT",
  lang    = "nl",
  intro   = "Automatisch samengesteld op " .. today,
  sections = {
    { title = "Open acties", columns = {"Taak","Deadline"},
      rows = {{"Leveranciersbeoordeling","2026-09-01"}},
      links = {"https://atlas.example/entity/taak/TASK-DEMO"} },
    { title = "Toelichting", body = "Een **korte** toelichting." },
  },
  footer = "Automatisch verzonden door Atlas.",
}
mail.send{ to = "maaike@example.nl", subject = "Wekelijks MT", html = html, text = text }
```

New file `internal/lua/mailrender.go` (binding + Lua-table →
`mailrender.Message` conversion), registered from `registerMailModule` as a
**method value** per the existing contextcheck note.

**BaseURL plumbing — the one genuine wrinkle.** `safeHref` resolves relative
links against `Options.BaseURL`, which lives in `mail.Config`; `lua` cannot
import `mail`. Following the `RecipientPolicyCarrier` precedent exactly, add an
**optional** interface in `internal/lua`:

```go
// BaseURLCarrier is an OPTIONAL capability a MailSender may implement.
type BaseURLCarrier interface{ MailBaseURL() string }
```

The wiring site's sender already holds the config and satisfies it structurally.
A sender that does not implement it yields an empty BaseURL, which `safeHref`
already handles by dropping relative links (text renders unlinked). No widening
of `MailSender`, no new constructor return value, no cycle.

**Registration policy:** `mail.render` is registered **unconditionally**,
exactly like `mail.send`, and works even when mail is not configured — it is
pure formatting with no transport, so `not_configured` would be nonsense. This
is consistent with the reasoning in `mail.go`'s package doc.

**Error convention:** `mail.render` **raises** on every failure. Unlike
`mail.send`, nothing here is network-bound — a render failure is always a
malformed argument, i.e. a bug in the script. An invalid `lang` is therefore a
raise, not a silent fallback: a script that means `"nl"` and typed something
else should be told.

**Files to modify:**

| File | Change |
|---|---|
| `internal/mailrender/template.go` | A1–A6: table-ise headings, spacer row, roles/scope, `lang` attr, `@media` dark block, logo dimensions |
| `internal/mailrender/mailrender.go` | `Message.Lang`, `Options.DefaultLang`, lang validation, `LogoWidth/Height`, dark palette tokens |
| `internal/mailrender/testdata/digest.golden.{html,txt}` | regenerate |
| `internal/mailrender/compat_test.go` | **new** — dataset-driven compatibility check |
| `internal/mailrender/testdata/caniemail.min.json` | **new** — pinned dataset fixture |
| `internal/mailtemplate/mailtemplate.go` | `Lang` field on `Template` (yaml `lang:`), carried into the built `Message` |
| `internal/lua/mailrender.go` | **new** — `mail.render` binding + table conversion |
| `internal/lua/mail.go` | register `render`; add `BaseURLCarrier` |
| `internal/lua/mailrender_test.go` | **new** — table-driven binding tests |
| `.go-arch-lint.yml` | add `mailrender` to `lua.mayDependOn` with rationale comment |
| `docs/` (mail docs) | document `mail.render` and `lang:` |

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `subject`, `intro`, `body`, `footer` | Lua script / entity content — **UNTRUSTED** | goldmark → bluemonday, unchanged pipeline | markup stripped |
| table `rows` cells | Lua script — **UNTRUSTED** | `template.HTMLEscapeString`, never markdown-rendered (a cell is a value) | escaped |
| `links` | Lua script — **UNTRUSTED** | `safeHref` allowlist: `http://`/`https://` absolute, or `/`-relative resolved against BaseURL | dropped; text renders unlinked |
| **`lang`** | **operator config AND untrusted Lua** | BCP-47 **shape** allowlist in `mailrender` (where both paths converge) — it lands in an HTML attribute | **rejected**, never sanitized |
| `DefaultLang` | operator | same validator, at `New` | rejected at construction |
| dark palette tokens | operator | existing `ValidatePalette` colour allowlist | rejected at `New` |
| `LogoWidth/Height` | operator | non-negative ints, emitted only when > 0 | attribute omitted |

**Security-Sensitive Operations:**

- **The render pipeline order is the security property**, and Part B must not
perturb it: markdown → goldmark → bluemonday (**content only**) → trusted
template → douceur **last**. Part B reuses `Renderer.Render` wholesale rather
than reimplementing any step, so the order cannot drift on the new path.
- **`lang` is the one new value that reaches raw markup from an untrusted
source.** `html/template` would escape it, but escaping an invalid language tag
yields a *wrong document*, not a safe one — hence validate-and-reject, and do it
in `mailrender` so neither call site can be bypassed.
- **The binding must never grow a raw-HTML or raw-CSS field.** That is precisely
the hole `mail.send{html=...}` already has and that this path exists to offer an
alternative to. Recorded in the package doc so a future widening is a conscious
act.
- **Dark-mode CSS is trusted template text with palette-validated values only** —
the `@media` block is authored in the template, never assembled from caller
data. douceur does zero CSS value validation and runs last, so this is
non-negotiable.
- **No new credential, file, or network surface.** `mail.render` performs pure
in-memory formatting; it does not dial, read the filesystem, or touch secrets.
- Errors carry only the offending field name and Lua type — never content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|---|---|
| 1 (no `padding` off-cell) | `compat_test.go` parses rendered output, asserts no `padding` on a non-`td`/`th` element |
| 2 (table spacing survives) | assert a spacer row follows `.tbl`; assert `.tbl` has no bottom `margin` |
| 3 (roles/scope) | assert `.tbl` lacks `role="presentation"`, its `<th>` carry `scope="col"`, `.wrap`/`.card` keep theirs |
| 4 (`lang` per message) | **the key test:** ONE `Renderer`, two `Render` calls with `Message.Lang` `"nl"` and `"en"` → two different `lang` attributes. Plus: empty `Message.Lang` falls back to `Options.DefaultLang`; both empty → `"en"`; never `lang=""` |
| 5 (logo dims) | with dimensions set, `<img>` carries `width`/`height`; unset omits both |
| 6 (dark mode) | assert `@media (prefers-color-scheme: dark)` survives to final output — **already proven feasible by experiment** |
| 7 (regression guard) | inject a `<div style="padding:1px">` fixture through the compat checker and assert it FAILS — a guard that cannot fail is not a guard |
| 8 (existing assertions) | the 22 existing tests pass unchanged; goldens regenerated and diffed by eye |
| 9 (byte-identical) | build one model (**including `Lang`**); render via `mailrender` directly and via `mail.render`; `require.Equal` both parts |
| 10 (sanitization) | `<script>`, `onerror=`, `javascript:` href in script markdown → absent from output |
| 11 (link safety) | `javascript:`, `data:`, and relative-with-no-BaseURL links dropped, text still present |
| 12 (ordering canary) | output contains inline `style=` attributes |
| 13 (raise vs return) | malformed call raises; delivery failure still returns an error table |
| 14 (arch-lint) | `just arch-lint` passes; `go list -deps` confirms no cycle |
| 15 | `just test`, `just lint`, `just coverage-check` |

Additionally, an end-to-end test through `mailtemplate`: a `mail_templates:`
entry with `lang: nl` produces a message whose rendered HTML carries `lang="nl"`
— proving the value survives config → `Message` → template.

**Integration approach:** AC9 is the real integration test — it pins the two
paths together, so a future change to `mailrender` that the Lua binding fails to
track breaks the build rather than silently producing different mail. Plus an
end-to-end test through the `memory` transport asserting a script's
`mail.render` + `mail.send` lands a message with both parts populated.

**Edge Cases:**

- Empty `sections` table → renders subject/intro/footer only, no stray markup.
- Section with `columns` but zero `rows` → the existing "Nothing to show." note.
(Note this string is template-authored English and does NOT follow `lang` — a
known wart, explicitly out of scope, but assert it renders so the gap is visible
rather than surprising.)
- Ragged rows (fewer cells than columns) → must not panic; short rows render
short rather than crashing.
- `links` shorter than `rows` → trailing rows unlinked (documented existing
behaviour of `buildSection`).
- Unicode + emoji in subject and cells → `TestRender_UnicodeSubjectAndBody`
covers the renderer; re-assert through the binding.
- `lang` casing (`"NL"`, `"nl-NL"`, `"zh-Hant-TW"`) → accepted, emitted verbatim.
- Very large table (10k rows) → must not blow the Lua stack; bounded conversion.
- `nil`/missing optional fields → treated as empty, not an error.
- Lua table with non-consecutive integer keys → `Len()` semantics; assert
behaviour rather than assuming.
- CR/LF in subject → **not** this binding's job; `internal/mail` rejects it at
enqueue. Assert it still does via the binding path.

**Negative Tests:**

- `mail.render` with no argument, a non-table argument, `subject` missing or
non-string, `sections` not a table, a section that is not a table, `rows`
containing a non-string cell → each **raises** with a message naming the field.
- `lang = "nl\" onload=x"`, `lang = "../../etc"`, `lang = "<script>"`, an
over-long tag → **rejected**, both from Lua (raises) and from `mail_templates:`
(load error). Explicitly assert the attribute-breakout attempt is refused rather
than escaped.
- Palette with `url('javascript:alert(1)')` → rejected at `New` (existing
`TestNew_RejectsBadPalette`, extended to the dark tokens).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Golden-file churn hides a security regression.** Table-ising the headings
rewrites much of the golden HTML, so a reviewer skimming the diff could miss a
real change. *Mitigation:* the security properties are asserted by explicit
content assertions (`TestRender_KeepsInlineStyles`,
`TestRender_PreservesMSOConditionals`,
`TestRender_NoDangerousCSSReachesStyleAttributes`), not by golden diffing. Those
tests are not touched.
2. **The Lua binding becomes a sanitizer bypass** if it later grows a raw-HTML
field. *Mitigation:* no such field now; the package doc records why, so adding
one is deliberate.
3. **`lua -> mailrender` reads as inverting the `mail -> lua` arrow.**
*Mitigation:* arch-lint comment states mailrender is a leaf (verified by `go
list -deps`) and the arrows are independent.
4. **`lang` gets re-added to `Options` by a future change**, quietly restoring
the one-language-per-deployment bug. *Mitigation:* AC4's
two-renders-one-renderer test fails if the value stops being per-message; the
field's godoc says why it is on `Message`.
5. **Dataset goes stale.** *Mitigation:* it is a compatibility **floor**, not a
source of truth; the fixture records version and `last_update_date`.
6. **Dark mode looks wrong in a client that partially honours it.**
*Mitigation:* overrides use `!important` on a small, explicit set of tokens
(bg/card/text/border) rather than restyling wholesale; clients ignoring the
at-rule get today's light rendering unchanged.
7. **Coverage floor on `internal/mailrender`** (security-relevant, floored in
`.testcoverage.yml`). *Mitigation:* the work adds substantially more test than
code.

**Effort:** `l` — Part A is contained, Part B adds a binding plus an optional
interface and a cross-path equivalence test; `lang` touches three packages but
shallowly.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] Mail docs — document `mail.render`, its table shape, the sanitization
contract, that it works without mail configured, and the `lang:` key on
`mail_templates:` plus the operator-level default.
- [x] `CLAUDE.md` — the mail bullet already states the pipeline order; extend it
with the `lua -> mailrender` direction and why it does not invert `mail -> lua`.
- [x] Package docs — `internal/lua/mailrender.go` (why no raw-HTML field) and
`internal/mailrender` (dark-mode block is trusted/palette-validated; `Lang` is
on `Message` not `Options`, and why).
- [x] ~~`docs/metamodel.md`, `docs/cli-reference.md`, `docs/data-entry.md`,
`README.md`~~ (N/A: no metamodel, CLI, UI, or project-level change).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the plan was
reviewed by the user directly, twice, and both rounds changed it materially —
see below)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** No `/design-review` agent run. The plan went through
two rounds of direct user review instead, and both produced substantive
corrections that a design review is meant to catch — so the phase did its job
even though the command did not run. Recorded honestly rather than checked off
as if the agent had run.

**Round 1 — `lang` on the wrong type.** The plan put the language on
`Options`. The user pushed back: "lang should be param right? not all mail will
be nl." That was correct and the plan was wrong. `Options` is renderer-scoped
and a `Renderer` is built once per deployment, so an `Options.Lang` would have
stamped one language on every mail an instance sends. Moved to `Message`, with
`Options.DefaultLang` as the fallback. This is now an invariant in CLAUDE.md and
is pinned by a test that renders twice from ONE renderer.

**Round 2 — dark mode was backwards.** The user asked "what is best practise on
dark-mode, just dont do it?" and supplied the Litmus guide. The plan had
proposed `<meta name="color-scheme">` as a harmless extra for the two clients
that honour it. Litmus names that exact thing as a trap: Apple Mail applies a
partial invert when the tag is present *without* a full dark stylesheet, and
Apple Mail otherwise leaves this template alone. The plan would have shipped a
regression in the best-behaved major client. Scope was rewritten to defensive
only — no meta tag, a `@media` block for the tier where the query verifiably
works, palette hardening for the tier that cannot be targeted at all. AC6 now
asserts the tag's ABSENCE so the trap cannot be re-entered.

Both rounds are the failure mode design review exists for: a plausible-looking
decision that is wrong for a reason not visible from the code. Neither would
have been caught by implementation or by code review.
