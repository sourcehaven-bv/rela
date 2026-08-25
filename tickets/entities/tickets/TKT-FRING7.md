---
id: TKT-FRING7
type: ticket
title: Focus rings are a hardcoded indigo that ignores the theme, and vanish entirely in High Contrast
kind: enhancement
priority: medium
effort: s
status: in-progress
---

## Goal

Two defects with one root cause — the SPA's focus rings are hand-written
`box-shadow` literals rather than a token — fixed in one pass:

1. **Wrong colour.** Focus rings render a hardcoded indigo that is not this
   app's accent and does not follow the theme.
2. **No ring at all in forced-colors mode.** `outline: none` plus a
   `box-shadow` ring means Windows High Contrast users get no focus indicator,
   because forced-colors drops `box-shadow` entirely.

Both were found and fixed for `CheckboxWidget` in TKT-CBSTYLE
([[RR-CBS2QW]], [[RR-CBS3AC]]). That fix was deliberately scoped to one
widget. This ticket applies it to the rest.

## Evidence

Counted on `develop` at time of writing.

**Hardcoded indigo** — `grep -rn "rgba(99, 102, 241"` etc:

| pattern | count | renders? |
| ------- | ----- | -------- |
| `rgba(99, 102, 241, …)` live literal | **25** | **yes** |
| `var(--accent-color, #6366f1)` fallback | 47 | no — `--accent-color` is always defined |
| bare `#6366f1` | 17 | 16 are the `palette.ts` default palette; 1 real (`ConflictsView.vue:517`) |

The app's real accent is `#4772fb` (light) / `#6f93ff` (dark), so the 25 live
literals paint an off-hue ring in both themes. They also defeat operator
theming: `@layer rela` exists so an unlayered `custom.css` accent wins, but a
literal is unreachable by it.

Alpha values are near-uniform, which is what makes this tractable:

| alpha | count |
| ----- | ----- |
| `0.1` | **21** |
| `0.25` / `0.2` / `0.06` / `0.05` | 1 each |

**Forced-colors**: `outline: none` appears **21** times outside tests, **18**
of them paired with a `box-shadow` ring, across 18 files. `forced-colors`
appears exactly once in `src/` — the `CheckboxWidget` block added by
TKT-CBSTYLE.

## Scope

- Add a focus-ring token to `tokens.css` (colour — that file's contract),
  derived from `--accent-color` so it follows the theme and operator overrides.
- Replace the **25 live literals** with it.
- Give the ~18 `outline: none` + `box-shadow` sites a `forced-colors` fallback
  that restores a real `outline`. A shared class or a single rule is preferred
  over 18 copies — decide during planning.
- Keep `CheckboxWidget`'s existing fix working; it is the reference
  implementation, and its `color-mix` approach is already proven in the build.

## Non-goals

- **The 47 `var(--accent-color, #6366f1)` fallbacks.** They never render, since
  `--accent-color` is unconditionally defined for both themes. Churning 47
  sites for zero visual change is not worth the review cost. Worth doing as a
  separate mechanical sweep if ever.
- **`palette.ts` / `palette.test.ts`.** Those 16 `#6366f1` hits are a
  legitimate default palette value, not a hardcoded style. A careless
  find-and-replace would break palette assignment — explicitly out.
- **No visual redesign.** Ring geometry (`0 0 0 2px`) stays as authored; only
  the colour source and the forced-colors behaviour change.
- **Not a general accessibility audit.** Only the focus-ring/forced-colors
  interaction. Contrast ratios, tab order, ARIA and screen-reader behaviour are
  out.
- **`tokens.css` is byte-identical to `internal/dataentry/apps_tokens.css`**
  and pinned by `TestAppTokensCSSInSyncWithFrontend`. Any token added must be
  mirrored there in the same commit, or CI fails.

## Risks

- **`tokens.css` is a cross-boundary contract.** It is served to custom apps as
  `_rela.css`. Adding a token widens what apps can rely on, so the name matters
  more than usual.
- **`color-mix` support.** Proven to survive this build (TKT-CBSTYLE ships it),
  but if a static token value is preferred, per-theme literals in `tokens.css`
  are the zero-risk alternative.
- **Wide, shallow diff.** ~25 sites across 18 files. Mostly mechanical, but
  every touched control needs its focus state confirmed rather than assumed —
  the same trap that let a specificity bug ship in TKT-CBSTYLE under green
  tests.
