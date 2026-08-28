---
id: DOCS-CUSTOMFOLDER
type: docs-checklist
title: 'Docs: custom/ folder layout and arbitrary asset serving'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] `openCustomEntry` godoc states the NESTED `os.OpenRoot` is
      security-critical, with the concrete case a single root would miss
      (a symlink to an in-project file outside `custom/`).
- [x] `validCustomEntry` godoc separates the two checks and states plainly that
      the dot rule contributes ZERO traversal defence — `path.Clean` resolves
      `..` before it runs.
- [x] `customAssetExists` godoc explains stat-not-read, the readability probe,
      and the accepted TOCTOU.
- [x] `shellVariants` godoc carries the 2²=4 trip-wire for a future third entry.
- [x] `custom.go` package godoc states the route is PUBLIC and UNAUTHENTICATED
      and wider than `apps/`.
- [x] `#nosec G705` rationale rewritten — its old stated boundary (the two-name
      allowlist) no longer exists.

## Project Documentation

- [x] `internal/dataentry/CLAUDE.md` — section retitled to `custom/`; the
      inverted "two allowlisted filenames" bullet replaced with the real
      boundary; new bullets on nested-root criticality, traversal being
      NEUTRALIZED not rejected, the dot rule's actual scope, and the public
      route.
- [x] `internal/dataentryconfig/config.go` — `DisableCustomInjection` comments.
- [x] `frontend/relaCssLayer.ts` — comment URL.
- [x] ~~`frontend/CLAUDE.md`~~ (N/A: names `custom.css`, still accurate.)

## User-facing Documentation

- [x] `docs/customisation.md` — folder tree, the `url(/_custom/logo.svg)`
      example, and the **false claim removed**: "Only these two exact filenames
      are served... no way to serve any other project file" was wrong in both
      halves.
- [x] Exposure warning placed **above** the layout example, not in the bottom
      Notes block: everything in `custom/` is published, unauthenticated, and
      more exposed than `apps/`. States plainly that the dot rule is a crude
      filename check, not a secrets scanner (`notes.md`, `custom.css~` served).
- [x] No-index-resolution and unknown-type-as-download behaviour documented.
- [x] `docs-project/entities/guides/GUIDE-data-entry.md` (generated-doc SOURCE)
      updated + `just docs` run; `docs/data-entry.md` regenerated.
