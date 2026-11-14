---
id: TKT-JZ304S5
type: ticket
title: Scheduled-tasks guide uses peer naming instead of remotes
kind: chore
priority: low
effort: xs
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

The scheduled-tasks guide said `peers.yaml`, `peer:<name>` and linked to a
missing `peer-sync.md`. The code uses `remotes.yaml` (`remotesync.FileName`),
the `remote:` task prefix (`scheduler.RemoteTaskPrefix`) and the guide
`remote-sync.md`. Fix the guide source in `docs-project/` and regenerate
`docs/`.

GitHub issue: #2166
