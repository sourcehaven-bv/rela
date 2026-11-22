---
id: RR-0N40ZR0
type: review-response
title: The shell keeps the boot values until a restart
finding: bootShellHandler and shellBoot read app.name and app.language once at startup. An operator who edits data-entry.yaml sees the old name on the setting-up screen until the server restarts, while tenant pages pick the change up on reload.
severity: minor
resolution: 'Kept: the shell must read no project file per request, and a change to the shared config already needs a rollout. docs/postgres-backend.md and its guide say the values are read at startup.'
status: addressed
---
