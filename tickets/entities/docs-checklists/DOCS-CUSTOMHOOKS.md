---
id: DOCS-CUSTOMHOOKS
type: docs-checklist
title: 'Docs: operator customisation hooks (custom.css / custom.js)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Package-level godoc on `internal/dataentry/custom.go` covering the trust model —
      explicitly contrasting `custom.js` (fully trusted, same-origin, no CSP) with `apps/`
      (sandboxed iframe, path-scoped CSP), so nobody reasons from one to the other.
- [x] Godoc on `injectTags` recording WHY it splices rather than parse+render with
      `golang.org/x/net/html` (a round-trip normalises the whole document and would be the first
      `html.Render` in `internal/`).
- [x] Godoc on `customAssetExists` recording that it stats rather than reads, and documenting the
      residual TOCTOU with the subsequent fetch as accepted rather than overlooked.
- [x] Doc comment on `frontend/relaCssLayer.ts` explaining the cascade problem, both carve-outs
      (`:root` tokens, `!important` inversion), and why postcss replaced the regex.
- [x] Test doc comments name the ticket/finding they pin (TKT-3DBK6I, RR-XOTMPN, RR-6J7SO7).

## Project Documentation

- [x] `internal/dataentry/CLAUDE.md` — new section: the SPA-shell rewrite as the ONE server-side
      HTML rewrite, justified on the SECURITY-boundary argument with an explicit CSP trip-wire;
      splice-don't-parse; four-variants-plus-restat; the two-name allowlist; the trust-model warning.
- [x] `frontend/CLAUDE.md` — the `@layer rela` convention (tokens stay outside, don't hand-write
      `@layer`, `!important` inverts, the wrap is build-only) and the SFC-not-string-template
      testing trap.
- [x] ~~Root `CLAUDE.md`~~ (N/A: no new package, no architectural boundary change.)
- [x] ~~`docs/metamodel.md`, `docs/cli-reference.md`, `README.md`~~ (N/A: no metamodel, CLI, or
      project-level surface change.)

## User-facing Documentation

- [x] New `docs/customisation.md` — the three-tier contract table, the verbatim stability
      disclaimer, how to write `custom.css` (including the `!important` inversion and why tokens
      are unlayered), how to write `custom.js`, the fully-trusted-vs-sandboxed comparison table,
      both documented gotchas (DOM inside `#app` is destroyed; the SPA mounts after your module,
      with a working `MutationObserver` recipe), the `<rela-slot>` contract, and
      `disable_custom_injection`.
- [x] `docs/data-entry.md` — cross-reference from the Styles section, framing this as the escape
      hatch and the palette/theme system as the supported path.
- [x] `<rela-slot>` documented honestly as **reserved, not yet emitted** (RR-CR-SLOTUNUSED), so an
      operator does not define one and wonder why nothing renders.
