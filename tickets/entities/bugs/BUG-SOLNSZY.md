---
id: BUG-SOLNSZY
type: bug
title: Settings and About gating, a read-only workspace without pile changes
description: The Settings page offers shared controls the server refuses to users without settings:edit and shows internal property names and file paths; Settings and About cannot be hidden per space or role; a read-only workspace offers pile changes it refuses; the About dialog shows nothing useful.
priority: medium
effort: m
why1: The Settings page rendered every card for every user and did not know whether the user may save; the sidebar footer always showed Settings and About; the pile controls checked only that piles are available; the About dialog showed only the app name and description.
why2: The Settings page was built for a single-user project where the user owns the configuration, and the footer buttons predate the builtins visibility setting.
why3: The saas-dev config pass with a customer portal was the first to show Settings to users without settings:edit and to use a read-only workspace with piles.
why4: No test opened Settings as a user without settings:edit, and the pile tests did not cover a read-only workspace.
why5: /_me reported no settings permission, so the client had nothing to gate on; the footer buttons were not part of the builtins rows.
prevention: /_me reports can_edit_settings from the same check the save routes use; settings and about are builtins rows; usePiles has one writable flag for every pile control. Go, vitest and e2e tests pin each case.
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Found in the saas-dev config pass with a customer portal.

- A portal customer without `settings:edit` sees the create-form defaults,
palette, logo and theme package cards, with schema property names and
`.rela/*.yaml` paths. Every save is refused.
- Settings and About cannot be hidden from a space or a role.
- In a read-only workspace "Add to pile", "Add to new pile…", the pile
toggles and the pile page's rename, delete and remove are offered and refused.
- The About dialog shows the app name and nothing else.

## Acceptance Criteria

- [x] Without `settings:edit`, or in a read-only workspace, Settings shows only the user's own preferences and no internal names or paths.
- [x] `builtins.settings` and `builtins.about` hide the footer buttons per space or permission.
- [x] A read-only workspace offers no pile change.
- [x] `app.about` adds a version and links to the About dialog; with nothing to show the About button is hidden.
- [x] The behaviour is documented.
