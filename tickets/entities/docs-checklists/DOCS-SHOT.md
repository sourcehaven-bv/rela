---
id: DOCS-SHOT
type: docs-checklist
title: 'Documentation: rela-docs phase 3 screenshot{} (TKT-89X2B5)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Package godoc on `internal/docscapture` (browser-backed capture; kept separate from core docs to isolate the browser dep) and the `docs.Capturer` interface + `CaptureSpec`/`Annotation` DTOs (`internal/docs/screenshot.go`)
- [x] Godoc on the non-obvious seams — `project.syncSeed` (seed staleness / DR-S2), `renderabilityGate` (DR-S4, the form-state gate), `annotateScript` (json.Marshal injection-safety / DR-C2), sandbox-first `newBrowser` (DR-M1)
- [x] Inline comments for the fixture-vs-real seed replay, the per-request role resolver, the height cap + CaptureBeyondViewport

## Project Documentation

- [x] User-facing guide updated — `docs-project/entities/guides/GUIDE-rela-docs.md` gained a **Screenshots** section (args table, `as=` roles, the Chrome + built-SPA prerequisites, fail-loud behavior); regenerated to `docs/rela-docs.md` via `just docs`
- [x] Example manual updated — `prototypes/data-entry/manual/tickets-manual.md` gained a `screenshot{}` figure that builds end-to-end
- [x] CLAUDE.md — ~~pointer to internal/docscapture~~ (N/A: the doc-language subsystem is documented in its own guide + package godoc; no new cross-cutting convention beyond what the guide covers)

## External Documentation

- [x] CLI reference — `rela docs build` already documented; `screenshot{}` is an island in the manual language (covered by the guide), not a new command/flag
- [x] ~~API reference~~ (N/A: no wire/HTTP API change — the data-testid is an internal test hook)
