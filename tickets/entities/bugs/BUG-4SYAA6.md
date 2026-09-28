---
id: BUG-4SYAA6
type: bug
title: History, restore and history-purge ignore faces
description: HTTP restore and CLI history/restore/purge act on the zero face, so faced entities have no usable history and cannot be erased.
priority: high
effort: m
status: backlog
---

## Problem

Reported by the face-awareness inventory, not yet verified:

- HTTP history restore is face-blind (`history_handler.go:138`, `history_restore.go:39,57`).
- CLI `history`, `restore` and `history-purge` pass the zero face (`history.go:42`, `restore.go:39`, `history_purge.go:52-59,124-127`).
- The short-form wrappers `ListVersions`, `GetVersion` and the purge requests hard-code face `""`.

The purge gap blocks compliance erasure of a faced entity's history.

## Expected

Every history, restore and purge surface takes an `ID@face` address and acts on
that face's lineage.
