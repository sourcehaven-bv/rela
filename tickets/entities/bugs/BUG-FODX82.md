---
id: BUG-FODX82
type: bug
title: Desktop app never starts the live-update feed
description: cmd/rela-desktop builds the data-entry App and router (main.go ~467-518) but never calls App.StartWatching; only cmd/rela-server/serve.go:211 does. On desktop the SSE stream connects but carries no entity:changed frames, external .md edits are not watched, and data-entry.yaml changes do not reload.
priority: medium
effort: s
status: backlog
---
