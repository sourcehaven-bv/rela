---
id: TKT-FGIWPE
type: ticket
title: rela-desktop runs on the SQLite backend and opens self-contained projects
kind: enhancement
priority: high
effort: m
started: "2026-10-04"
completed: "2026-10-04"
status: done
---

## Description

Build rela-desktop with the sqlite tag, recognise a project whose config lives
only in rela.db, check for data-entry.yaml through the config loader rather than
os.Stat, and add File menu items to export and import config.

## Acceptance criteria

- just build-desktop / install-desktop produce a sqlite build.
- Opening a self-contained project works from the welcome screen, recent list and Finder.
- Export/Import config from the File menu round-trips.
