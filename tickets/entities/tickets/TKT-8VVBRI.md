---
id: TKT-8VVBRI
type: ticket
title: 'Design tokens: spacing, radius, typography and elevation scales for the data-entry SPA'
kind: enhancement
priority: medium
effort: m
status: review
---

## Description

**PR 1 of 3** splitting TKT-5V8704 (see FEAT-OJ8L0H). Frontend-only, no config
surface — lands first so the layout and icon PRs consume tokens rather than
re-touching the same literals.

`frontend/src/styles/tokens.css` is a well-documented single source of truth for
*color*, shared verbatim with the Go binary. It stops at color, so everything
else has drifted. Measured across `frontend/src/**/*.{vue,css}`:

- **7 border-radius values** — 2/3/4/6/8/10/12/16px (85x `6px`, 61x `4px`,
29x `8px`) where three steps would do.
- **18 font sizes** — 10px through 26px (110x `14px`, 86x `13px`, 55x `12px`,
37x `11px`) where a ~6-step scale would do.
- **Spacing mixes units** — 8/12/16/6/4px dominate, but `0.6rem`, `0.4rem`,
`0.3rem`, `0.25rem` appear alongside.
- No shadow or elevation tokens at all.

### Constraints (from design review RR-09N4MN)

- **The typography contract lives in Go, not CSS.** `appTypographyCSS`
(`internal/dataentry/apps_css.go:83-95`, emitted at `:126`) froze
`--font-family` + `--font-size-{sm,base,lg,xl}` in TKT-PF4E6S. The type scale
must **reuse those names**, not shadow them, and the guarding test belongs
Go-side. `tokens.css` holds colour only and cannot break the font contract.
- **Respect the `tokens.css` boundary.** Its doc comment states *"Only theme
tokens belong here (a stable, near-unbreakable contract). Layout, components,
and behavior do not."* Non-theme scales therefore go in a sibling
(`styles/scales.css`) imported from `main.ts`.

### Scope control

Migrate the high-traffic surfaces (detail, list, form, kanban); **no repo-wide
sweep**. Record the literal census before/after rather than chasing zero — the
goal is a scale that exists and is used, not perfect purity.

Note the three `.properties-list` components carry 19 radius/font literals
between them (`SectionEditForm` 3, `PropertyDisplay` 2, `SidePanel` 14). Those
are migrated here so PR 2 can restructure layout without also churning values.

## Acceptance criteria

1. Spacing, radius, typography and elevation scales are defined in a documented
location that respects the `tokens.css` theme-only boundary.
2. The type scale reuses the existing `--font-size-*` names from the Go
`appTypographyCSS` contract rather than introducing parallel names.
3. A **Go-side** test pins that `appTypographyCSS` still emits `--font-family`
and the `--font-size-*` scale (guards TKT-PF4E6S).
4. Detail-page, list, form and kanban surfaces consume the new tokens; the
distinct radius and font-size literal counts drop substantially from the 7 / 18
measured above, with before/after counts recorded.
5. Both light and dark themes verified visually across the affected views.
6. No regression in frontend unit tests, Go tests, or the e2e suite.

## Out of scope

- Layout restructuring (PR 2, TKT-5V8704) and icons (PR 3).
- Label humanization — separate PR entirely.
