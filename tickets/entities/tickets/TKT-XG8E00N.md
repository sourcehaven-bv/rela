---
id: TKT-XG8E00N
type: ticket
title: Port the operator palette to rela-components tokens
kind: refactor
priority: medium
effort: m
status: backlog
---

## Description

The data-entry SPA now paints with the library's `--rl-*` tokens (TKT-ME8LEI),
but the operator palette still produces rela's retired token names. Port it:

- `dataentryconfig.deriveTheme` derives the full palette from the 8 base colors. It must emit `--rl-*` tokens. `rlTokens` maps only accent and badges today; the other roles need fg/bg pairs that keep text at WCAG AA contrast.
- `applyPalette` in `frontend/src/stores/ui.ts` must set the same `--rl-*` names.

## Acceptance criteria

1. `TestPaletteCarriesEveryDefaultToken` in `internal/dataentry/apps_test.go` runs again (no `t.Skip`) and passes.
2. A configured palette changes the `--rl-*` tokens the SPA reads, in light and dark mode.
3. A project without a palette looks the same as before.
