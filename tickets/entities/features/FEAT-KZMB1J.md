---
id: FEAT-KZMB1J
type: feature
title: Wails v3 desktop shell
summary: The desktop app runs on Wails v3, supports several windows onto a project, opens .rela bundles from Finder, and refuses to run twice against the same project.
description: 'Move rela-desktop from Wails v2 to v3 and build on what v3 makes available: multiple windows, single-instance handoff, window state, and Finder integration for .rela project bundles.'
priority: medium
status: in-progress
---

## Why

Wails v2 gave the desktop shell one window, no shutdown hook and no instance
lock. v3 supplies all three, plus a dock/badge and notification surface the
scheduler can eventually drive.

The migration was cheap because the Wails surface is small and the Vue SPA has
no Wails coupling at all — it talks HTTP to a Go handler and does not know it is
in a desktop shell.

## Scope

- v3 migration at parity, v2 removed from `go.mod`
- multi-window: cmd/middle-click a link, or File > New Window
- single-instance lock with encrypted IPC handoff
- window geometry persisted between runs
- `.rela` project bundles registered with Finder
- `just install-desktop`

## Out of scope

Dock badges and native notifications: both need a Developer ID signature rather
than the current ad-hoc signing. Linux, which v3 moves to GTK4/WebKitGTK 6.0 and
where the dock service is a no-op stub.
