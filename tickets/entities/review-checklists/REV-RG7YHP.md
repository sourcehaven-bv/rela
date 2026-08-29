---
id: REV-RG7YHP
type: review-checklist
title: 'Review: Extend the icon set to a curated ~150 names, generate registry + docs from one source, add icon: none'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` exit 0; 2021 frontend tests pass
- [x] Lint clean (`just lint`) — clean after fixing 21 findings the review surfaced (see below)
- [x] Comment lint gate clean (`just comment-lint`) — 11132 comments, no unresolvable doc links
- [x] Coverage maintained (`just coverage-check`) — exit 0

Also run: `just arch-lint` (OK), `just plimsoll` (OK), `just lint-md` (254
files, 0 issues), `vue-tsc -b` (clean), `npm run lint` (0 errors), `npm run
build`.

**Lint findings my own diff introduced**, all fixed rather than suppressed:
US-vs-UK spelling throughout (this repo uses US — one of them, `organisation`,
was a *config-facing icon name*, so that was a rename not a comment fix), plus
`intrange`, `modernize` (`slices.Contains`, `strings.SplitSeq`), `perfsprint`
(`errors.New`), `staticcheck` ST1005, and `gofmt`.

One `//nolint:funlen` on `icondefs.All`, with a reason: it is a 217-row **data
table**, and splitting it into helpers to satisfy a length limit would scatter
one list across a dozen functions and break the category grouping that *is* the
documentation order — no reduction in complexity, real loss of readability.

**Comment findings.** `just comment-report` reports nothing in any file this
ticket adds or touches.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

The reviewer found **four must-fix issues**, and — the part worth recording —
**two of them were design-review findings I had marked addressed but had only
half-fixed**. I verified every claim before acting; all four reproduced.

| ID | Sev | Finding | Fix |
| --- | --- | --- | --- |
| RR-1YIOG5 | critical | A shipped docs example used `icon: progress`, which the allowlist rejects. Pre-existing rot, but this ticket regenerated a table right beside it — and a *generated* table makes the hand-written examples around it look machine-checked, so the reader now trusts them more | Added `progress` (a real gap between `active` and `done`), and added `TestDocsExamplesUseValidNames` extracting every `icon:` from the guide and the rendered docs. Mutation-verified. Closes the class, not the instance |
| RR-8D1X16 | significant | `icon: None` suggested `"done"` — the new error message recommended a green check mark for the one typo it was built to catch. `NoIcon` isn't in `ValidIconNames`, so it was never a suggestion candidate | `suggestIcon` now includes `NoIcon`: suggestion cares what is legal to *write*, not what draws a glyph |
| RR-1AKC31 | significant | 5 of 11 "chrome" names guarded nothing (zero SPA importers — dead exports), while the *real* Go-side coupling was 7 bare string literals in the handler, entirely unguarded | Split into `spaChromeNames` (SPA imports, get named exports) and `DerivedNames` (server-emitted, named in Go). Added `TestDerivedIconsAreValidNames` asserting against the allowlist, not a literal |
| RR-82519A | significant | `icon: none` on an **action** entry drew a *document* glyph in the collapsed sidebar — a button that fires a mutation, indistinguishable from a link to a document, with no confirm dialog. And the test enshrined it by asserting only "an svg exists" | Actions derive their own glyph (`zap`), which removes the empty-fallback case rather than special-casing it. Tests now pin *which* glyph, by Lucide class |
| RR-B6QV5W | minor | Five smaller items: a `TestValidate` subtest that could not fail (and shared input with a case asserting the opposite), byte-wise edit distance with no ASCII guard, a doc comment defending a case no entry exhibits, `-check` conflating missing-vs-stale, and kanban prose contradicting the sidebar prose 250 lines away | All fixed. The docs contradiction mattered most: a reader reconciling the two would "simplify" `none` → `""` and silently un-fix RR-4P3WPD and RR-D8I2R2 in one commit |

**Naming (L10).** Renamed the non-grandfathered use-site names —
`deadline`→`calendar-check` (a *checked* calendar reads "done", not "due"),
`identity`→`fingerprint`, `guide`→`book-open`, `priority`→`flame`, plus
`contract`→`gavel`, `medical`→`stethoscope`, `lab`/`experiment`→`beaker`/`flask`
from my own earlier pass. The four grandfathered ones (`dashboard`, `apps`,
`document`, `warning`) stay — they ship, and the no-regression test pins them.

Adopted the reviewer's best suggestion: the generated docs table now carries a
**Glyph column** naming the Lucide component. It self-disambiguates the
confusable families (five `Circle*` glyphs differ only in their interiors at
18px) and makes a use-site name visible as a mismatched row during review.

**Deferred, recorded not silently dropped (L11).** The reviewer noted
`DerivedIcon` is the second-best wire shape — `icon` + `suppressIcon: true`
would need no "populated only when suppressed" invariant and would make
RR-82519A impossible by construction. The current shape works and is tested; the
invariant is the kind a future refactor breaks quietly, which is exactly what
RR-82519A was. Worth revisiting if that field grows a third case.

**Unrelated changes:** none. The diff touches only icon definition, generation,
validation, rendering, their tests, and the docs/config that describe them. The
`.go-arch-lint.yml` and `.testcoverage.yml` edits are new-package registrations
this change requires.

## Acceptance Verification

All nine acceptance criteria PASS. Evidence is recorded per-criterion in
IMPL-ANVDGM; the load-bearing ones:

- **AC 1/2 (single source, drift fails):** verified by tampering with a
generated file and watching `TestGeneratedFilesAreUpToDate` fail with a message
naming `just generate-icons`; restored and re-ran green.
- **AC 3 (set size):** 217 icons, all verified to resolve against the installed
`lucide-vue-next@1.0.0`.
- **AC 4 (docs table):** `just generate-icons` → guide entity → `just docs` →
`docs/data-entry.md`, chained correctly, now with a Glyph column.
- **AC 5/6 (`none`):** `TestIconNoneEndToEnd` asserts the actual JSON, where the
three intents are distinct — the property an empty-string encoding would have
destroyed.
- **AC 8 (no regression):** all 16 original names pinned in Go and TS, with
component identity asserted.
- **AC 9 (security):** `ICONS` stays a closed statically-imported map; the
prototype-key fallback tests are untouched; `none` never reaches `resolveIcon`.

**Bundle cost:** +51,047 bytes raw (+0.8%) across all assets, measured by
building both sides. Within the estimate; no trimming needed.
