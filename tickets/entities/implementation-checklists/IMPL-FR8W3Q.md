---
id: IMPL-FR8W3Q
type: implementation-checklist
title: 'Implementation: Focus rings are a hardcoded indigo that ignores the theme, and vanish entirely in High Contrast'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written~~ (N/A: the integration surface is rendered
      pixels. jsdom does not apply scoped SFC styles and there is no visual
      baseline suite, so the meaningful automated assertion is source-level —
      see Test Quality — backed by in-browser checks)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place~~ (N/A: CSS tokens have no failure mode)

**What changed**

| change | count |
| ------ | ----- |
| `--focus-ring` token added (`tokens.css` + Go mirror) | 1 |
| `--error-ring` token added | 1 |
| focus rings → `var(--focus-ring)` | 20 |
| error rings → `var(--error-ring)` | 6 |
| background tints → `color-mix` off the right token | 4 |
| global `forced-colors` fallback stylesheet | 1 new file |
| guard tests | 7 |

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A)
- [x] Property comparisons use original object, not hardcoded strings

The guards read the **source tree**, deliberately. A mounted-component
assertion on `getComputedStyle(...).boxShadow` cannot fail here — Vitest does
not apply an SFC's scoped `<style>`, so it resolves against no CSS at all and
passes whatever the stylesheet actually says. TKT-CBSTYLE shipped a real
cascade bug underneath exactly that kind of green test; this avoids repeating
it. Offenders are reported with `file:line` and the offending text so a failure
is actionable rather than a bare boolean.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- **AC1 — token follows the theme.** Verified in-browser BEFORE writing the
  code, because the whole approach depends on it: a `color-mix`/`var()` token
  declared only in `:root` re-resolves under `:root.dark`, giving
  `#4772fb` → `#6f93ff` with no second declaration. That measurement is why
  the token has no `:root.dark` twin. PASS
- **AC2 — no literals remain.** `grep` for the rgba forms returns 0 outside
  comments and dead `var()` fallbacks; pinned by 3 guard tests. PASS
- **AC3 — forced-colors.** PARTIAL, and stated as such rather than claimed.
  What IS verified: the rule ships in the served bundle exactly as authored —
  `@media (forced-colors:active){:focus-visible{outline-offset:2px;outline:2px
  solid highlight}}` — the browser recognises the media feature
  (`matchMedia('(forced-colors: active)').media !== 'not all'`), and 3 tests
  pin the rule's presence, its import from `main.ts`, and the absence of
  `!important`. What is NOT verified: rendering under an actual High Contrast
  session. That needs a Windows HCM environment (or a devtools emulation this
  tooling cannot reach), so it was not observed. The mechanism is the same one
  TKT-CBSTYLE shipped for the checkbox.
- **AC4 — no other visual change.** 1839 frontend tests pass; the
  `tokens.css` ↔ `apps_tokens.css` byte-sync test and the frozen-typography
  contract tests all pass. PASS

**Contrast measured, not eyeballed.** The planning assumption was "swap the
hue, keep the look". Computing WCAG 2.2 §1.4.11 ratios against the composited
surfaces showed every translucent option failing the 3:1 non-text minimum —
the old ring scored **1.13:1**, and even 30% alpha only reached 1.46. Full
opacity clears it at **4.14 light / 5.85 dark**. Raised as a scope decision
rather than decided unilaterally; the user chose the WCAG-passing ring.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Three things the guard test caught that the survey missed.** Worth recording,
because it is the argument for writing the guard before declaring the sweep
done:

1. **Six error-state rings** (`rgba(239, 68, 68, …)`) with the identical
   defect — a red that is not `--error-color`. Not in the original count.
   Fixed with `--error-ring`; excluding them would have left the ticket
   half-done for no reason.
2. **Four `background:` tints** matched the same literal but are NOT rings.
   Swapping them to the opaque token would have painted solid accent blocks
   over text. They keep a translucent `color-mix` off the right token.
3. **The `!important` check matched its own explanatory comment.** A correct
   file failed. Now checks declarations only.

**On scope creep.** The error rings are a widening of the ticket as written.
Justified: same root cause, same fix, same files, and the guard that enforces
the focus half would otherwise have to deliberately ignore them. The dead
`var(--x, rgba(...))` fallbacks stay out — they cannot render, and ~47 edits
would bury the real diff.

**Token naming.** `--focus-ring` / `--error-ring` are aliases of
`--accent-color` / `--error-color` rather than direct uses of them, so ring
contrast can be revisited (or given a dedicated hue) without repainting every
accent-coloured surface. They land in `tokens.css`, which is the theme-colour
contract and is served to custom apps — mirrored byte-for-byte into
`internal/dataentry/apps_tokens.css` in the same commit.
