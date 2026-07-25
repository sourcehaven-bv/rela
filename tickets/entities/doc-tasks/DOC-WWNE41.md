---
id: DOC-WWNE41
type: doc-task
title: Fold docs/transforms.md into the docs-project generation pipeline
description: docs/transforms.md was hand-authored during TKT-JF5JI8 because the docs/*.md files are generated from docs-project/entities/ and no source entity existed. Create a GUIDE-transforms entity (or fold the content into GUIDE-export) in docs-project/ so `just docs` owns the file, then remove the standalone hand-maintained copy. Noted as an accepted N/A in DOCS-FIY2UG.
priority: low
status: backlog
---

## Task

`docs/transforms.md` (view-export / transform registry reference: registering
transforms, CLI `rela render`, data-entry entity+list export, `export_render`
override, security notes, v1 limits) is currently a standalone hand-maintained
file, unlike the rest of `docs/*.md`, which `scripts/generate-docs.sh` (`just
docs`) generates from `docs-project/entities/`.

Steps:

1. Create `GUIDE-transforms` in `docs-project/entities/guides/` carrying the transforms.md content (or fold it into `GUIDE-export` if one page is preferred), with the proper `path:` so generation emits `docs/transforms.md`.
2. Wire concept/feature relations in docs-project (export feature, data-entry, metamodel).
3. Run `just docs` and verify the generated file replaces the hand-authored one 1:1.
4. Remove any now-stale hand-edits; from then on edits go through docs-project.
