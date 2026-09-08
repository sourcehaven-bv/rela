---
id: IMPL-FJC421
type: implementation-checklist
title: 'Implementation: Mail template compatibility hardening + a mailrender-backed render path for Lua'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**Part A — template (`internal/mailrender`)**

| Change | Files |
|---|---|
| A1 section headings + empty note are single-cell tables, not padded divs | `template.go` |
| A2 `.tbl` margin → spacer row (`.gap`) | `template.go` |
| A3 `role="presentation"` off `.tbl`, `scope="col"` on its `<th>`; layout tables keep the role | `template.go` |
| A4 per-message `Message.Lang` + `Options.DefaultLang` + `ValidateLang` | `mailrender.go`, `template.go` |
| A5 defensive `@media` dark block; **no** `color-scheme` meta; palette hardened | `template.go` |
| A6 logo `width`/`height` from `Options.LogoWidth/LogoHeight` | `mailrender.go`, `template.go` |
| Dataset-driven compat suite + pinned fixture | `compat_test.go`, `testdata/caniemail.min.json` |

**Part B — Lua path**

| Change | Files |
|---|---|
| `mail.render{...} -> html, text` | `internal/lua/mailrender.go` (new) |
| `BaseURLCarrier` optional interface | `internal/lua/mailrender.go` |
| `MailBaseURL()` on the real sender | `internal/mail/script.go` |
| `lang:` on declarative templates, validated at load | `internal/mailtemplate/mailtemplate.go` |
| `lua -> mailrender` allowed, with rationale | `.go-arch-lint.yml` |

**Two findings discovered during implementation, both fixed:**

1. **`.pad table` leaked onto the new section scaffolding.** The markdown-table
rules were keyed on `.pad`, and the new section tables live inside `.pad`, so
they inherited cell borders and padding. Fixed by scoping those rules to a
`.prose` class applied at the three places sanitized markdown lands (intro,
section body, footer). Recorded in the template doc so the selectors are not
rewritten back onto `.pad`.

2. **A `<ul>` carried `padding-left: 20px`** — a genuine A1 offender the initial
sweep missed. Lists cannot become table cells without mangling author markup, so
the indent moved to `margin-left`, which Outlook honors on a list while dropping
padding. This inverts the general rule, so it is documented as the exception.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`compatSample` is the single builder every compat check renders, so the checks
cannot drift onto different documents. `TestMailRender_MatchesDirectRenderer`
compares against a model built in the test rather than a pasted string, so it
stays honest when the template changes. The dark-bar test reads both colors out
of the rendered CSS and compares them to each other rather than asserting a
literal hex, so retuning the palette does not falsely fail it.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Rendered an Atlas-shaped message (the one from Mailpit `5TvadkwnFjwGC9xxIj6O2T`)
through the new `mail.render` binding and opened the result in a browser, in
both schemes.

- **Dark mode** (Chrome in dark mode, the `@media` block active): dark card on
darker page, legible body text, visible table rules, accent-blue link, muted
column headers. Screenshot reviewed.
- **Light mode** (dark block stripped, simulating the ~60% of clients that
ignore the query): identical structure, original light palette. Screenshot
reviewed.
- **Text alternative**: generated automatically, correct section headings and
the resolved absolute URL.

**A defect was found by looking rather than by any assertion.** The first dark
rule was `.wrap, .wrap td { background-color: darkBg !important }` — a
descendant selector that hit *every* cell inside the wrapper, including the 4px
accent bar, repainting it the background color and erasing the only brand color
in the layout. Every structural test still passed. Fixed by targeting `.outer`
and `.bar` explicitly, and pinned by
`TestCompat_DarkModeDoesNotEraseTheAccentBar`, which asserts the bar's color
differs from the page background rather than asserting a literal.

Per-criterion:

| AC | Result |
|---|---|
| 1 padding only on cells | PASS — `TestCompat_PaddingOnlyOnTableCells`; zero-value resets exempted |
| 2 table spacing survives | PASS — `TestCompat_NoLayoutMarginOnStructuralElements` |
| 3 roles / scope | PASS — `TestCompat_DataTableIsNotPresentational` |
| 4 per-message lang | PASS — `TestRender_LangIsPerMessage` (two renders, ONE renderer), fallback + no `lang=""` |
| 5 logo dimensions | PASS — `TestCompat_LogoCarriesIntrinsicDimensions` |
| 6 dark block survives, no meta tag | PASS — `TestCompat_DarkModeBlockSurvivesInlining`, `TestCompat_NoColorSchemeMetaTag` |
| 7 guard actually fires | PASS — `TestCompat_GuardActuallyFires` feeds it a padded `<div>` and asserts detection |
| 8 existing assertions intact | PASS — all pre-existing tests pass; goldens reviewed line by line, HTML nesting checked balanced |
| 9 byte-identical paths | PASS — `TestMailRender_MatchesDirectRenderer` |
| 10 sanitization | PASS — `TestMailRender_SanitizesUntrustedContent` |
| 11 link safety | PASS — `TestMailRender_LinkSafety` (5 schemes), `TestMailRender_NoBaseURLDropsRelativeLinks` |
| 12 ordering canary | PASS — `TestMailRender_KeepsInlineStyles` |
| 13 raise vs return | PASS — `TestMailRender_MalformedCallRaises` (12 cases) |
| 14 arch-lint | PASS — `just arch-lint` "OK - No warnings found" |
| 15 test/lint/coverage | PASS — see Quality |

**One AC-8 note worth recording:** `TestRender_KeepsInlineStyles` previously
asserted `<style>` was gone entirely. That is no longer correct — the `@media`
block cannot be inlined and must survive. Rather than weaken the assertion, it
now strips the at-rule and asserts nothing survives *outside* it, so it still
fails if inlining stops running. Weakening it to "contains `style=`" would have
retired the canary.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

- `BaseURLCarrier` mirrors `RecipientPolicyCarrier` exactly (optional interface,
same file-level rationale) rather than widening `MailSender`.
- `mail.render` is registered as a **method value**, matching the existing
`contextcheck` note on `registerMailModule`.
- `lang` validation lives in `mailrender`, the one point both the operator path
and the untrusted-Lua path pass through — validating at either call site would
have left the other open.
- No new raw-HTML or raw-CSS surface; `TestMailRender_NoRawHTMLField` asserts an
`html =` field is not honored, so the absence is pinned rather than trusted.
- Verification commands: `go test ./...` (all pass), `just lint` (0 issues),
`just arch-lint` (no warnings), `just comment-lint` (no unresolvable doc links),
`just coverage-check` (exit 0). Affected-package coverage: mailrender 90.6%
(floor 85), lua 86.4% (floor 80), mail 86.4% (floor 85), mailtemplate 85.5%.
- Scratch generator tests used during development were removed; no debug code
remains.
