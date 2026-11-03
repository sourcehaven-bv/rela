---
id: TKT-FLTR9P
type: ticket
title: SPA hides the git sync and settings save controls from principals without permission
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

BUG-OFW8S0 made `POST /api/v1/_git/sync` need `git:sync` and the settings-page
saves need `settings:edit`. The server check is authoritative, but the SPA still
shows the controls to everyone:

- `GitStatus` in `frontend/src/api/git.ts` has no `can_sync` field, and `SidebarFooter.vue` always renders the sync button. A principal without `git:sync` clicks it and gets a 403.
- `GET /api/v1/_settings` exposes no `can_edit`, so the settings page cannot hide or disable Save. `saveSettings` shows a generic error and drops the 403 detail that names `settings:edit`.

## Approach

Add `can_sync` to `GitStatus` and render the sync control only when it is true.
Add a `can_edit` flag to the settings GET response, gate Save and the palette,
logo and theme-import controls on it, and show the problem+json `detail` on a
failed save.
