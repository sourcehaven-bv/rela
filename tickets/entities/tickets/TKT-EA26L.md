---
id: TKT-EA26L
type: ticket
title: Remove dead htmx templates and vendor-js justfile target after Vue migration
kind: chore
priority: low
effort: xs
status: done
---

After the Vue SPA migration (FEAT-24hp) the htmx-era assets are no longer wired
in. Clean up the leftovers.

## Scope

1. Delete `internal/dataentry/templates/form.html` and `_partials.html` — orphaned, no Go code references them; the Vue SPA at `internal/dataentry/static/v2/` is the only active UI.
2. Remove the `vendor-js` target from `justfile` (lines ~232–243). It downloads `htmx.min.js` plus 7 other libs (easymde, slim-select, tagify, cytoscape, mermaid, …) into `internal/dataentry/static/`. None of those files are checked in anymore (only `favicon.svg` and `v2/` remain), and the Vue frontend bundles these via npm in `frontend/package.json`.
3. Update the `data-entry-ui` concept description to reflect the current Vue/SPA architecture instead of "Go HTML templates / HTMX for dynamic updates".

## Out of scope

- Vue SPA changes
- Any UI/UX behavior changes

No functional change — pure dead-code/hygiene cleanup.
