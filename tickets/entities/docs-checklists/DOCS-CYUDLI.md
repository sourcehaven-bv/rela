---
id: DOCS-CYUDLI
type: docs-checklist
title: 'Docs: Named dashboards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: `dashboard.go` (the `default` alias, yes/no guard), `DashboardView.vue` (generation guard; why named dashboards keep NextActionCard and analysis), Configure `DashboardView.vue` (`?name=` and unknown-name note), `commands.go` (qualifier `default`).
- [x] Function/type docs if public API: `DashboardRef`, `DefaultDashboardName`, `DashboardByName`, `AllDashboards`, `NamedDashboard`.

## Project Documentation

- [x] README updated (if applicable): not applicable.
- [x] CLAUDE.md updated (if new patterns): not applicable; no new repo pattern.
- [x] Help text accurate (if CLI changes): no CLI changes.
- [x] User guide: new section "Named dashboards (`dashboards`)" in `docs-project/entities/guides/GUIDE-data-entry.md`; nav table, space `home:`, page tab and `available_on` entries updated; `docs/data-entry.md` regenerated with `just docs`.

## External Documentation

- [x] Changelog entry added: the repo keeps no changelog file; release notes come from PR titles.
- [x] API docs updated (if applicable): OpenAPI `_config` schema gains `dashboards` (golden files regenerated); `/api/v1/_dashboard?name=` and its 404 `dashboard_not_found` are described in the guide.
